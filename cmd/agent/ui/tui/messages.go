package tui

import "github.com/ryanreadbooks/tokkibot/cmd/agent/ui/types"

// Tea messages for event handling

type (
	// ContentMsg represents streaming content from assistant
	ContentMsg struct {
		Round            int
		Content          string
		ReasoningContent string
	}

	// ToolCallMsg represents a tool call event
	ToolCallMsg struct {
		Round     int
		Name      string
		Arguments string
	}

	// ClearRoundMsg signals the end of a conversation round
	ClearRoundMsg int

	// TokensUpdateMsg triggers token count refresh
	TokensUpdateMsg struct{}

	// ShellConfirmMsg is an alias for types.ShellConfirmRequest
	ShellConfirmMsg = types.ShellConfirmRequest

	// ErrorMsg represents an error event
	ErrorMsg error
)
