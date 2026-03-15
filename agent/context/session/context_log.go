package session

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/ryanreadbooks/tokkibot/agent/ref"
	"github.com/ryanreadbooks/tokkibot/llm/schema/param"
)

// ContextLog manages in-memory context for LLM, with support for reset, compress and flush.
type ContextLog struct {
	baseLog

	mu   sync.RWMutex
	logs []LogItem
}

func (s *ContextLog) AddLogItem(item LogItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logs = append(s.logs, item)
	return s.writeLine(&item)
}

func (s *ContextLog) GetLogs() []LogItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.logs
}

func (s *ContextLog) ResetLogsFromMessage(messages []param.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	newLogs := make([]LogItem, 0, len(messages))
	for _, p := range messages {
		msg := p
		newLogs = append(newLogs, s.newLogItem(p.Role(), &msg))
	}
	s.logs = newLogs
}

func (s *ContextLog) Flush(root string) error {
	path := s.logFilePath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	if s.f == nil {
		return nil
	}

	if err := s.f.Truncate(0); err != nil {
		return err
	}
	if _, err := s.f.Seek(0, 0); err != nil {
		return err
	}

	s.mu.RLock()
	snapshot := slices.Clone(s.logs)
	s.mu.RUnlock()

	for _, item := range snapshot {
		if content := item.Json(); len(content) > 0 {
			if _, err := s.f.WriteString(content + "\n"); err != nil {
				return err
			}
		}
	}

	return nil
}

// CompressToolCalls compresses the first N eligible tool call messages to ref files.
func (s *ContextLog) CompressToolCalls(count int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	compressed := 0
	for i := range s.logs {
		if compressed >= count {
			break
		}

		item := &s.logs[i]
		if item.Role != param.RoleTool || item.Message == nil || item.Message.Tool == nil {
			continue
		}

		toolMsg := item.Message.Tool
		content := toolMsg.GetContent()

		if isAlreadyRef(content) || len(content) <= 500 {
			continue
		}

		refName, err := ref.Save(content)
		if err != nil {
			continue
		}

		toolMsg.String = &param.String{
			Value: refName + " (use load_ref tool to read full content)",
		}
		toolMsg.Texts = nil
		compressed++
	}

	return compressed, nil
}

func (s *ContextLog) GetUncompressedToolCallCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for i := range s.logs {
		item := &s.logs[i]
		if item.Role != param.RoleTool || item.Message == nil || item.Message.Tool == nil {
			continue
		}
		content := item.Message.Tool.GetContent()
		if len(content) > 500 && !isAlreadyRef(content) {
			count++
		}
	}
	return count
}

func (s *ContextLog) initFile(workspace string) error {
	if err := s.baseLog.initFile(workspace, os.O_CREATE|os.O_RDWR|os.O_APPEND); err != nil {
		return err
	}
	return s.loadExistingLogs(workspace)
}

func (s *ContextLog) loadExistingLogs(workspace string) error {
	items, err := readLogItems(s.logFilePath(workspace))
	if err != nil {
		return err
	}
	if len(items) > 0 {
		s.mu.Lock()
		s.logs = items
		s.mu.Unlock()
	}
	return nil
}

func isAlreadyRef(content string) bool {
	if !strings.HasPrefix(content, ref.RefPrefix) {
		return false
	}
	return strings.Contains(content, "(use load_ref tool to read full content)") || len(content) < 100
}
