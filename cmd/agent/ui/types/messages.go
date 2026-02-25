package types

import "time"

// ShellConfirmRequest represents a shell confirmation request
type ShellConfirmRequest struct {
	Command string
	RespCh  chan bool
}

// MessageRole represents the role of a message sender
type MessageRole int

const (
	RoleUser MessageRole = iota
	RoleAssistant
	RoleToolCall // Tool call execution
)

// Message represents a chat message in the UI
type Message struct {
	Role             MessageRole
	Content          string
	ReasoningContent string
	Timestamp        time.Time
	
	// For tool calls (only when Role == RoleToolCall)
	ToolName      string
	ToolArguments string
	ToolComplete  bool
}

// IsUser returns true if message is from user
func (m *Message) IsUser() bool {
	return m.Role == RoleUser
}

// IsAssistant returns true if message is from assistant
func (m *Message) IsAssistant() bool {
	return m.Role == RoleAssistant
}

// IsToolCall returns true if message is a tool call
func (m *Message) IsToolCall() bool {
	return m.Role == RoleToolCall
}
