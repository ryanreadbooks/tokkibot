package session

import (
	"fmt"
	"sync"
)

type Log interface {
	*AOFLog | *ContextLog
	initFile(workspace string) error
	closeFile()
}

type LogManager[T Log] struct {
	Workspace string
	newLog    func(channel, chatId string) T

	mu   sync.RWMutex
	logs map[string]T
}

func NewLogManager[T Log](workspace string, newLog func(channel, chatId string) T) *LogManager[T] {
	return &LogManager[T]{
		Workspace: workspace,
		newLog:    newLog,
		logs:      make(map[string]T),
	}
}

func NewAOFLogManager(workspace string) *LogManager[*AOFLog] {
	return NewLogManager(workspace, func(channel, chatId string) *AOFLog {
		return &AOFLog{
			baseLog: baseLog{
				filename: "log.jsonl",
				channel:  channel,
				chatId:   chatId,
			},
		}
	})
}

func NewContextLogManager(workspace string) *LogManager[*ContextLog] {
	return NewLogManager(workspace, func(channel, chatId string) *ContextLog {
		return &ContextLog{
			baseLog: baseLog{
				filename: "log.context.jsonl",
				channel:  channel,
				chatId:   chatId,
			},
		}
	})
}

func (m *LogManager[T]) Get(channel, chatId string) T {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.logs[logKey(channel, chatId)]
}

// GetOrCreate returns an existing log or lazily creates and initializes a new one.
func (m *LogManager[T]) GetOrCreate(channel, chatId string) (T, error) {
	key := logKey(channel, chatId)

	m.mu.RLock()
	if log, ok := m.logs[key]; ok {
		m.mu.RUnlock()
		return log, nil
	}
	m.mu.RUnlock()

	log := m.newLog(channel, chatId)
	if err := log.initFile(m.Workspace); err != nil {
		return log, fmt.Errorf("failed to init log file: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.logs[key]; ok {
		log.closeFile()
		return existing, nil
	}

	m.logs[key] = log
	return log, nil
}

func logKey(channel, chatId string) string {
	return channel + "_" + chatId
}
