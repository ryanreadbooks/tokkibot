package cron

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	chmodel "github.com/ryanreadbooks/tokkibot/channel/model"
)

const (
	metaFileName   = "meta.json"
	promptFileName = "prompt.md"
)

// Task represents a cron task
type Task struct {
	Name      string     `json:"name"`
	CronExpr  string     `json:"cron_expr"`
	Enabled   bool       `json:"enabled"`
	CreatedAt time.Time  `json:"created_at"`
	LastRun   *time.Time `json:"last_run,omitempty"`

	// delivery settings
	Deliver        bool         `json:"deliver"`
	DeliverChannel chmodel.Type `json:"deliver_channel,omitempty"`
	DeliverTo      string       `json:"deliver_to,omitempty"`

	// runtime fields, not persisted
	prompt  string     `json:"-"`
	taskDir string     `json:"-"`
	mu      sync.Mutex `json:"-"` // prevent concurrent execution
}

// TaskOption for configuring a task
type TaskOption func(*Task)

// WithDelivery enables delivery to a channel
func WithDelivery(channel chmodel.Type, to string) TaskOption {
	return func(t *Task) {
		t.Deliver = true
		t.DeliverChannel = channel
		t.DeliverTo = to
	}
}

// NewTask creates a new cron task
func NewTask(name, cronExpr, prompt string, opts ...TaskOption) *Task {
	t := &Task{
		Name:      name,
		CronExpr:  cronExpr,
		Enabled:   true,
		CreatedAt: time.Now(),
		prompt:    prompt,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// ChatId returns the fixed chat id for cron task session
func (t *Task) ChatId() string {
	return t.Name
}

// Prompt returns the task's prompt content
func (t *Task) Prompt() string {
	return t.prompt
}

// SetPrompt sets the task's prompt content
func (t *Task) SetPrompt(prompt string) {
	t.prompt = prompt
}

// Save persists the task to disk
func (t *Task) Save(cronsDir string) error {
	taskDir := filepath.Join(cronsDir, t.Name)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return fmt.Errorf("failed to create task directory: %w", err)
	}
	t.taskDir = taskDir

	// save meta.json
	metaPath := filepath.Join(taskDir, metaFileName)
	metaData, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal task meta: %w", err)
	}
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return fmt.Errorf("failed to write task meta: %w", err)
	}

	// save prompt.txt
	promptPath := filepath.Join(taskDir, promptFileName)
	if err := os.WriteFile(promptPath, []byte(t.prompt), 0644); err != nil {
		return fmt.Errorf("failed to write task prompt: %w", err)
	}

	return nil
}

// UpdateMeta updates only the meta.json file
func (t *Task) UpdateMeta(cronsDir string) error {
	taskDir := filepath.Join(cronsDir, t.Name)
	metaPath := filepath.Join(taskDir, metaFileName)
	metaData, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal task meta: %w", err)
	}
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return fmt.Errorf("failed to write task meta: %w", err)
	}
	return nil
}

// LoadTask loads a task from a directory
func LoadTask(taskDir string) (*Task, error) {
	// load meta.json
	metaPath := filepath.Join(taskDir, metaFileName)
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read task meta: %w", err)
	}

	var task Task
	if err := json.Unmarshal(metaData, &task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task meta: %w", err)
	}
	task.taskDir = taskDir

	// load prompt.txt
	promptPath := filepath.Join(taskDir, promptFileName)
	promptData, err := os.ReadFile(promptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read task prompt: %w", err)
	}
	task.prompt = string(promptData)

	return &task, nil
}

// Delete removes the task from disk
func (t *Task) Delete(cronsDir string) error {
	taskDir := filepath.Join(cronsDir, t.Name)
	return os.RemoveAll(taskDir)
}
