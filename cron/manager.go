package cron

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/ryanreadbooks/tokkibot/config"
)

// TaskHandler is called when a cron task is triggered
type TaskHandler func(ctx context.Context, task *Task)

// Manager manages cron tasks
type Manager struct {
	cronsDir string
	cron     *cron.Cron
	tasks    map[string]*Task
	entryIDs map[string]cron.EntryID
	mu       sync.RWMutex
	handler  TaskHandler
}

// NewManager creates a new cron manager
func NewManager() *Manager {
	cronsDir := filepath.Join(config.GetWorkspaceDir(), "crons")
	return &Manager{
		cronsDir: cronsDir,
		cron:     cron.New(),
		tasks:    make(map[string]*Task),
		entryIDs: make(map[string]cron.EntryID),
	}
}

// SetHandler sets the task handler
func (m *Manager) SetHandler(handler TaskHandler) {
	m.handler = handler
}

// CronsDir returns the crons directory path
func (m *Manager) CronsDir() string {
	return m.cronsDir
}

// Load loads all tasks from the crons directory (data only, no scheduling)
func (m *Manager) Load() error {
	if err := os.MkdirAll(m.cronsDir, 0755); err != nil {
		return fmt.Errorf("failed to create crons directory: %w", err)
	}

	entries, err := os.ReadDir(m.cronsDir)
	if err != nil {
		return fmt.Errorf("failed to read crons directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		taskDir := filepath.Join(m.cronsDir, entry.Name())
		task, err := LoadTask(taskDir)
		if err != nil {
			slog.Warn("failed to load cron task", "dir", taskDir, "error", err)
			continue
		}

		m.mu.Lock()
		m.tasks[task.Name] = task
		m.mu.Unlock()
	}

	return nil
}

// ScheduleAll registers all enabled tasks with the cron scheduler
func (m *Manager) ScheduleAll() {
	// collect tasks first to avoid lock contention
	m.mu.RLock()
	tasksToSchedule := make([]*Task, 0)
	for _, task := range m.tasks {
		if task.Enabled {
			tasksToSchedule = append(tasksToSchedule, task)
		}
	}
	m.mu.RUnlock()

	// schedule outside the lock
	for _, task := range tasksToSchedule {
		if err := m.scheduleTask(task); err != nil {
			slog.Warn("failed to schedule cron task", "name", task.Name, "error", err)
		}
	}
}

// Start starts the cron scheduler
func (m *Manager) Start() {
	m.cron.Start()
	slog.Info("cron scheduler started", "tasks", len(m.tasks))
}

// Stop stops the cron scheduler
func (m *Manager) Stop() context.Context {
	return m.cron.Stop()
}

// scheduleTask registers a task with the cron scheduler
func (m *Manager) scheduleTask(task *Task) error {
	entryID, err := m.cron.AddFunc(task.CronExpr, func() {
		m.executeTask(task)
	})
	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	m.mu.Lock()
	m.entryIDs[task.Name] = entryID
	m.mu.Unlock()

	// calculate and log next run time
	if schedule, err := cron.ParseStandard(task.CronExpr); err == nil {
		nextRun := schedule.Next(time.Now())
		slog.Info("scheduled cron task", "name", task.Name, "expr", task.CronExpr, "next_run", nextRun)
	}

	return nil
}

// unscheduleTask removes a task from the cron scheduler
func (m *Manager) unscheduleTask(taskName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entryID, exists := m.entryIDs[taskName]; exists {
		m.cron.Remove(entryID)
		delete(m.entryIDs, taskName)
	}
}

// executeTask executes a cron task
func (m *Manager) executeTask(task *Task) {
	// prevent concurrent execution of the same task
	if !task.mu.TryLock() {
		slog.Warn("cron task already running, skipping", "name", task.Name)
		return
	}
	defer task.mu.Unlock()

	slog.Info("executing cron task", "name", task.Name)

	// update last run time
	now := time.Now()
	task.LastRun = &now
	if err := task.UpdateMeta(m.cronsDir); err != nil {
		slog.Error("failed to update task meta", "name", task.Name, "error", err)
	}

	if m.handler != nil {
		ctx := context.Background()
		m.handler(ctx, task)
	}
}

// AddOrUpdateTask adds a new cron task or updates an existing one
func (m *Manager) AddOrUpdateTask(task *Task) (updated bool, err error) {
	// validate cron expression
	if _, err := cron.ParseStandard(task.CronExpr); err != nil {
		return false, fmt.Errorf("invalid cron expression: %w", err)
	}

	m.mu.Lock()
	_, updated = m.tasks[task.Name]
	m.mu.Unlock()

	// save to disk
	if err := task.Save(m.cronsDir); err != nil {
		return updated, err
	}

	m.mu.Lock()
	m.tasks[task.Name] = task
	m.mu.Unlock()

	return updated, nil
}

// DeleteTask removes a cron task
func (m *Manager) DeleteTask(taskName string) error {
	m.mu.Lock()
	task, exists := m.tasks[taskName]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("task %s not found", taskName)
	}
	delete(m.tasks, taskName)
	m.mu.Unlock()

	// unschedule
	m.unscheduleTask(taskName)

	// delete from disk
	return task.Delete(m.cronsDir)
}

// EnableTask enables a cron task
func (m *Manager) EnableTask(taskName string) error {
	m.mu.Lock()
	task, exists := m.tasks[taskName]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("task %s not found", taskName)
	}
	task.Enabled = true
	m.mu.Unlock()

	if err := task.UpdateMeta(m.cronsDir); err != nil {
		return err
	}

	return m.scheduleTask(task)
}

// DisableTask disables a cron task
func (m *Manager) DisableTask(taskName string) error {
	m.mu.Lock()
	task, exists := m.tasks[taskName]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("task %s not found", taskName)
	}
	task.Enabled = false
	m.mu.Unlock()

	m.unscheduleTask(taskName)

	return task.UpdateMeta(m.cronsDir)
}

// GetTask returns a task by name
func (m *Manager) GetTask(taskName string) (*Task, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, exists := m.tasks[taskName]
	return task, exists
}

// ListTasks returns all tasks
func (m *Manager) ListTasks() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*Task, 0, len(m.tasks))
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// GetNextRun returns the next run time for a task
func (m *Manager) GetNextRun(taskName string) (time.Time, bool) {
	m.mu.RLock()
	entryID, exists := m.entryIDs[taskName]
	task, taskExists := m.tasks[taskName]
	m.mu.RUnlock()

	// if scheduled, use entry's next time
	if exists {
		entry := m.cron.Entry(entryID)
		return entry.Next, true
	}

	// otherwise, calculate from cron expression
	if taskExists && task.Enabled {
		schedule, err := cron.ParseStandard(task.CronExpr)
		if err == nil {
			return schedule.Next(time.Now()), true
		}
	}

	return time.Time{}, false
}
