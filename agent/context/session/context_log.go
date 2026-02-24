package session

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/ryanreadbooks/tokkibot/agent/ref"
	"github.com/ryanreadbooks/tokkibot/llm/schema"
)

// this is the log file for LLM context building
type ContextLog struct {
	baseLog

	mu sync.RWMutex
	// full log messages in memory
	logs []LogItem
}

// Flush writes all in-memory logs to disk
func (s *ContextLog) Flush(root string) error {
	// save all
	path := s.baseLog.fullLogFileName(root)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	if s.f != nil {
		if err := s.f.Truncate(0); err != nil {
			return err
		}

		s.mu.RLock()
		snapshot := slices.Clone(s.logs)
		s.mu.RUnlock()

		for _, msg := range snapshot {
			if content := msg.Json(); len(content) > 0 {
				if _, err := s.f.WriteString(content + "\n"); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (s *ContextLog) AddUserMessage(msg *schema.MessageParam) error {
	item := s.createLogItem(schema.RoleUser, msg)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, item)
	return s.baseLog.writeLine(&item)
}

func (s *ContextLog) AddAssistantMessage(msg *schema.MessageParam) error {
	item := s.createLogItem(schema.RoleAssistant, msg)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, item)
	return s.baseLog.writeLine(&item)
}

func (s *ContextLog) AddToolMessage(msg *schema.MessageParam) error {
	item := s.createLogItem(schema.RoleTool, msg)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, item)
	return s.baseLog.writeLine(&item)
}

// get full logs item
func (s *ContextLog) GetLogs() []LogItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.logs
}

// ResetLogsFromParam resets the in-memory logs with new params
func (s *ContextLog) ResetLogsFromParam(params []schema.MessageParam) {
	s.mu.Lock()
	defer s.mu.Unlock()

	newLogs := make([]LogItem, 0, len(params))
	for _, p := range params {
		msg := p
		newLogs = append(newLogs, s.baseLog.createLogItem(p.Role(), &msg))
	}

	s.logs = newLogs
}

// loadExistingLogs loads existing log items from file into memory
func (s *ContextLog) loadExistingLogs(path string) error {
	items, err := readLogItems(path)
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

// isAlreadyRef checks if content is already a ref reference
func isAlreadyRef(content string) bool {
	// Check if content starts with @refs/ and is relatively short (likely a ref)
	if strings.HasPrefix(content, ref.Prefix) {
		// If it contains the hint text, it's definitely a ref
		if strings.Contains(content, "(use load_ref tool to read full content)") {
			return true
		}
		// If it starts with @refs/ and is short (< 100 chars), likely a ref
		if len(content) < 100 {
			return true
		}
	}
	return false
}

// CompressToolCalls compresses the first N tool call messages to ref references
// Returns (compressed count, error)
func (s *ContextLog) CompressToolCalls(count int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	compressed := 0
	for i := range s.logs {
		if compressed >= count {
			break
		}

		if s.logs[i].Role == schema.RoleTool {
			if s.logs[i].Message != nil && s.logs[i].Message.ToolMessageParam != nil {
				toolMsg := s.logs[i].Message.ToolMessageParam
				originalContent := toolMsg.GetContent()

				// Skip if already a ref or content is too short
				if isAlreadyRef(originalContent) {
					continue
				}

				// Only compress if content is long enough
				if len(originalContent) > 500 {
					// Save to ref file
					refName, err := ref.Save(originalContent)
					if err != nil {
						continue
					}

					// Update message to use ref
					toolMsg.String = &schema.StringParam{
						Value: refName + " (use load_ref tool to read full content)",
					}
					toolMsg.Texts = nil
					compressed++
				}
			}
		}
	}

	return compressed, nil
}

// GetUncompressedToolCallCount returns the count of tool calls that can be compressed
func (s *ContextLog) GetUncompressedToolCallCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for i := range s.logs {
		if s.logs[i].Role == schema.RoleTool {
			if s.logs[i].Message != nil && s.logs[i].Message.ToolMessageParam != nil {
				content := s.logs[i].Message.ToolMessageParam.GetContent()
				// Count messages > 500 chars that are not already refs
				if len(content) > 500 && !isAlreadyRef(content) {
					count++
				}
			}
		}
	}
	return count
}

func (s *ContextLog) initFile(workspace string) error {
	// ContextLog uses O_APPEND for appending, but also supports flushing
	if err := s.baseLog.initFile(workspace, os.O_CREATE|os.O_RDWR|os.O_APPEND); err != nil {
		return err
	}

	// Load existing logs from file if any
	path := s.baseLog.fullLogFileName(workspace)
	return s.loadExistingLogs(path)
}
