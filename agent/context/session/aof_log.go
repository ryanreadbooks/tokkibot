package session

import (
	"errors"
	"fmt"
	"os"

	schema "github.com/ryanreadbooks/tokkibot/llm/schema"
)

func getSessionLogKey(channel, chatId string) string {
	return fmt.Sprintf("%s_%s", channel, chatId)
}

// Complete conversation AOFLog for a single chat
//
// - system workspace/sessions/channel/chatid/log.jsonl
//
// This is an AOF (Append-Only File)
type AOFLog struct {
	baseLog
}

func (s *AOFLog) retrieveLogItems(root string) ([]LogItem, error) {
	path := s.fullLogFileName(root)
	return readLogItems(path)
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
	item := s.createLogItem(schema.RoleUser, msg)
	return s.writeLine(&item)
}

func (s *AOFLog) AddAssistantMessage(msg *schema.MessageParam) error {
	item := s.createLogItem(schema.RoleAssistant, msg)
	return s.writeLine(&item)
}

func (s *AOFLog) AddToolMessage(msg *schema.MessageParam) error {
	item := s.createLogItem(schema.RoleTool, msg)
	return s.writeLine(&item)
}

func (s *AOFLog) initFile(workspace string) error {
	// AOF uses O_APPEND for append-only writes
	return s.baseLog.initFile(workspace, os.O_CREATE|os.O_APPEND|os.O_RDWR)
}
