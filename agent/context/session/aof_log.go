package session

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	schema "github.com/ryanreadbooks/tokkibot/llm/schema"
)

var compactThreshold = 5000

func getSessionLogKey(channel, chatId string) string {
	return fmt.Sprintf("%s_%s", channel, chatId)
}

// Complete conversation AOFLog for a single chat
//
// - system workspace/sessions/channel/chatid/log.jsonl
//
// This is an AOF file
type AOFLog struct {
	workspace string

	filename string

	channel string
	chatId  string

	f *os.File
}

func (f *AOFLog) closeFile() {
	if f.f != nil {
		f.f.Close()
	}
}

// ~/sessions/channel/chatid/log.jsonl
func (s *AOFLog) fullLogFileName(root string) string {
	return filepath.Join(root, s.channel, s.chatId, s.filename)
}

func (s *AOFLog) doRetrieve(path string) ([]LogItem, error) {
	f, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open session file: %w", err)
	}
	defer f.Close()

	items := make([]LogItem, 0, 128)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// one line is one log item
		var item LogItem
		err = json.Unmarshal(line, &item)
		if err != nil {
			continue
		}

		items = append(items, item)
	}

	return items, nil
}

func (s *AOFLog) retrieveLogItems(root string) ([]LogItem, error) {
	return s.doRetrieve(s.fullLogFileName(root))
}

func (s *AOFLog) checkExists(root string) error {
	path := s.fullLogFileName(root)
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("session not found")
	}

	return nil
}

func (s *AOFLog) AddUserMessage(msg *schema.MessageParam) error {
	item := LogItem{
		Id:      newLogItemId(),
		Role:    schema.RoleUser,
		Created: time.Now().Unix(),
		Message: msg,
	}

	return s.writeLine(&item)
}

func (s *AOFLog) AddAssistantMessage(msg *schema.MessageParam) error {
	item := LogItem{
		Id:      newLogItemId(),
		Role:    schema.RoleAssistant,
		Created: time.Now().Unix(),
		Message: msg,
	}

	return s.writeLine(&item)
}

func (s *AOFLog) AddToolMessage(msg *schema.MessageParam) error {
	item := LogItem{
		Id:      newLogItemId(),
		Role:    schema.RoleTool,
		Created: time.Now().Unix(),
		Message: msg,
	}

	return s.writeLine(&item)
}

func (s *AOFLog) writeLine(item *LogItem) error {
	str, err := json.Marshal(item)
	if err == nil && s.f != nil {
		str = append(str, '\n')
		_, err = s.f.Write(str)
		return err
	}

	return err
}

func (s *AOFLog) initFile(workspace string) error {
	path := s.fullLogFileName(workspace)
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return err
	}

	s.f = f

	return nil
}
