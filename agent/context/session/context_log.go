package session

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	// "time"

	"github.com/ryanreadbooks/tokkibot/llm/schema"
)

// this is the log file for LLM context building
type ContextLog struct {
	workspace string
	// log filename
	filename string

	f *os.File

	channel, chatId string

	mu sync.RWMutex
	// full log messages
	logs []LogItem
}

func (s *ContextLog) closeFile() {
	if s.f != nil {
		_ = s.f.Close()
	}
}

func (s *ContextLog) flush(root string) error {
	// save all
	path := s.fullLogFileName(root)
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return err
	}

	if s.f != nil {
		s.f.Truncate(0)
		s.mu.RLock()
		snapshot := slices.Clone(s.logs)
		s.mu.RUnlock()

		for _, msg := range snapshot {
			if content := msg.Json(); len(content) > 0 {
				_, err = s.f.WriteString(content + "\n") // ignore error here
				if err != nil {
					slog.Error("[session] failed to write session file", "error", err)
					return err
				}
			}
		}
	}

	return nil
}

func (s *ContextLog) logFileName() []string {
	return []string{s.channel, s.chatId, s.filename}
}

// ~/sessions/channel/chatid/log.context.jsonl
func (s *ContextLog) fullLogFileName(root string) string {
	elems := []string{root}
	elems = append(elems, s.logFileName()...)

	return filepath.Join(elems...)
}

func (s *ContextLog) AddUserMessage(msg *schema.MessageParam) error {
	item := LogItem{
		Id:      newLogItemId(),
		Role:    schema.RoleUser,
		Created: time.Now().Unix(),
		Message: msg,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, item)
	return s.writeLine(&item)
}

func (s *ContextLog) AddAssistantMessage(msg *schema.MessageParam) error {
	item := LogItem{
		Id:      newLogItemId(),
		Role:    schema.RoleAssistant,
		Created: time.Now().Unix(),
		Message: msg,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, item)
	return s.writeLine(&item)
}

func (s *ContextLog) AddToolMessage(msg *schema.MessageParam) error {
	item := LogItem{
		Id:      newLogItemId(),
		Role:    schema.RoleTool,
		Created: time.Now().Unix(),
		Message: msg,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, item)
	return s.writeLine(&item)
}

// get full logs item
func (s *ContextLog) GetLogs() []LogItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.logs
}

// override
func (s *ContextLog) ResetLogsFromParam(params []schema.MessageParam) {
	s.mu.Lock()
	defer s.mu.RUnlock()

	newLogs := make([]LogItem, 0, len(params))
	for _, p := range params {
		newLogs = append(newLogs, LogItem{
			Id:      newLogItemId(),
			Created: time.Now().Unix(),
			Role:    p.Role(),
			Message: &p,
		})
	}

	s.logs = newLogs
}

func (s *ContextLog) writeLine(item *LogItem) error {
	str, err := json.Marshal(item)
	if err == nil && s.f != nil {
		str = append(str, '\n')
		_, err = s.f.Write(str)
	}

	return err
}

// loadExistingLogs loads existing log items from file into memory
func (s *ContextLog) loadExistingLogs(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}
	if stat.Size() == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	decoder := json.NewDecoder(file)
	for {
		var item LogItem
		if err := decoder.Decode(&item); err == io.EOF {
			break
		} else if err != nil {
			slog.Warn("[session] failed to decode log item, skipping", "error", err)
			continue
		}
		s.logs = append(s.logs, item)
	}

	return nil
}

func (s *ContextLog) initFile(workspace string) error {
	path := s.fullLogFileName(workspace)
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	s.f = f

	// Load existing logs from file if any
	if err := s.loadExistingLogs(path); err != nil {
		return err
	}

	return nil
}
