package session

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type LogManagerConfig struct {
	Workspace    string
	SaveInterval time.Duration
}

type AOFLogManager struct {
	workspace string

	mu   sync.RWMutex
	logs map[string]*AOFLog
}

func NewAOFLogManager(ctx context.Context, c LogManagerConfig) *AOFLogManager {
	mgr := &AOFLogManager{
		logs:      make(map[string]*AOFLog),
		workspace: c.Workspace,
	}

	return mgr
}

func (s *AOFLogManager) Get(channel, chatId string) *AOFLog {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.logs[getSessionLogKey(channel, chatId)]
}

func (s *AOFLogManager) GetOrCreate(channel, chatId string) (*AOFLog, error) {
	s.mu.RLock()
	key := getSessionLogKey(channel, chatId)
	if sess, ok := s.logs[key]; ok {
		s.mu.RUnlock()
		return sess, nil
	}
	s.mu.RUnlock()

	// Create log instance and init file outside of write lock
	log := &AOFLog{
		baseLog: baseLog{
			filename:  "log.jsonl",
			workspace: s.workspace,
			channel:   channel,
			chatId:    chatId,
		},
	}

	if err := log.initFile(s.workspace); err != nil {
		return nil, fmt.Errorf("failed to init: %w", err)
	}

	// Double-check with write lock
	s.mu.Lock()
	defer s.mu.Unlock()
	if tmpSess, ok := s.logs[key]; ok {
		// Another goroutine already created it, close our file handle
		log.closeFile()
		return tmpSess, nil
	}

	// We are the first one, save it
	s.logs[key] = log
	return log, nil
}

func (s *AOFLogManager) GetLogItems(channel, chatId string) ([]LogItem, error) {
	log, err := s.GetOrCreate(channel, chatId)
	if err != nil {
		return nil, err
	}
	return log.retrieveLogItems(s.workspace)
}

type ContextLogManager struct {
	Workspace string

	mu   sync.RWMutex
	logs map[string]*ContextLog
}

func NewContextLogManager(ctx context.Context, c LogManagerConfig) *ContextLogManager {
	mgr := &ContextLogManager{
		Workspace: c.Workspace,
		logs:      make(map[string]*ContextLog),
	}

	return mgr
}

func (s *ContextLogManager) GetOrCreate(channel, chatId string) (*ContextLog, error) {
	s.mu.RLock()
	key := getSessionLogKey(channel, chatId)
	if sess, ok := s.logs[key]; ok {
		s.mu.RUnlock()
		return sess, nil
	}
	s.mu.RUnlock()

	// Create log instance and init file outside of write lock
	log := &ContextLog{
		baseLog: baseLog{
			filename:  "log.context.jsonl",
			workspace: s.Workspace,
			channel:   channel,
			chatId:    chatId,
		},
	}

	err := log.initFile(s.Workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to init: %w", err)
	}

	// Double-check with write lock
	s.mu.Lock()
	defer s.mu.Unlock()

	if tmpLog, ok := s.logs[key]; ok {
		// Another goroutine already created it, close our file handle
		log.closeFile()
		return tmpLog, nil
	}

	// We are the first one, save it
	s.logs[key] = log
	return log, nil
}
