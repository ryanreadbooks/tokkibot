package session

import (
	"encoding/json"
	"strings"
	// "unicode/utf8"

	"github.com/google/uuid"
	schema "github.com/ryanreadbooks/tokkibot/llm/schema"
)

func newLogItemId() string {
	id := uuid.Must(uuid.NewV7())
	return strings.ReplaceAll(id.String(), "-", "")
}

// LogItem every messages into session file
type LogItem struct {
	Id      string               `json:"id"` // unique msg id
	Role    schema.Role          `json:"role"`
	Created int64                `json:"created"`
	Message *schema.MessageParam `json:"message,omitzero"`
}

func (msg *LogItem) IsFromUser() bool {
	return msg.Role == schema.RoleUser
}

func (msg *LogItem) IsFromAssistant() bool {
	return msg.Role == schema.RoleAssistant
}

func (msg *LogItem) IsFromTool() bool {
	return msg.Role == schema.RoleTool
}

func (msg *LogItem) Json() string {
	c, _ := json.Marshal(msg)
	return string(c)
}
