package styles

import "github.com/charmbracelet/lipgloss"

// MessageTheme defines styling for a message type
type MessageTheme struct {
	HeaderStyle lipgloss.Style
	BodyStyle   lipgloss.Style
	BoxStyle    lipgloss.Style
}

// ToolCallTheme defines styling for tool call display
type ToolCallTheme struct {
	NameStyle lipgloss.Style
	ArgsStyle lipgloss.Style
}

// TokensTheme defines styling for token counter
type TokensTheme struct {
	TextStyle lipgloss.Style
}

// ConfirmTheme defines styling for confirmation dialog
type ConfirmTheme struct {
	BoxStyle  lipgloss.Style
	TextStyle lipgloss.Style
}

// Theme contains all UI styling
type Theme struct {
	User         MessageTheme
	Assistant    MessageTheme
	Thinking     MessageTheme
	ToolCallMsg  MessageTheme  // Tool call as message
	ToolCall     ToolCallTheme // Keep for compatibility
	Tokens       TokensTheme
	Confirm      ConfirmTheme
}

// DefaultTheme returns the default theme
func DefaultTheme() *Theme {
	return &Theme{
		User: MessageTheme{
			HeaderStyle: lipgloss.NewStyle().
				Foreground(ColorUserPrimary).
				Bold(true),
			BodyStyle: lipgloss.NewStyle().
				Foreground(ColorUserPrimary).
				AlignHorizontal(lipgloss.Left),
			BoxStyle: lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder(), true, true, true, true).
				BorderForeground(ColorUserPrimary).
				Padding(0, 1).
				MarginBottom(1),
		},
		Assistant: MessageTheme{
			HeaderStyle: lipgloss.NewStyle().
				Foreground(ColorAssistantPrimary).
				Bold(true),
			BodyStyle: lipgloss.NewStyle().
				Foreground(ColorAssistantPrimary).
				AlignHorizontal(lipgloss.Left),
			BoxStyle: lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder(), true, true, true, true).
				BorderForeground(ColorAssistantPrimary).
				Padding(0, 1).
				MarginBottom(1),
		},
		Thinking: MessageTheme{
			HeaderStyle: lipgloss.NewStyle().
				Foreground(ColorAssistantThinking),
			BodyStyle: lipgloss.NewStyle().
				Foreground(ColorAssistantThinking).
				AlignHorizontal(lipgloss.Left),
			BoxStyle: lipgloss.NewStyle(),
		},
		ToolCallMsg: MessageTheme{
			HeaderStyle: lipgloss.NewStyle().
				Foreground(ColorToolCall).
				Italic(true),
			BodyStyle: lipgloss.NewStyle().
				Foreground(ColorToolCallArgs).
				AlignHorizontal(lipgloss.Left),
			BoxStyle: lipgloss.NewStyle(),
		},
		ToolCall: ToolCallTheme{
			NameStyle: lipgloss.NewStyle().
				Foreground(ColorToolCall).
				Italic(true).
				Bold(true),
			ArgsStyle: lipgloss.NewStyle().
				Foreground(ColorToolCallArgs),
		},
		Tokens: TokensTheme{
			TextStyle: lipgloss.NewStyle().
				Foreground(ColorTokens),
		},
		Confirm: ConfirmTheme{
			BoxStyle: lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorConfirmDanger).
				Padding(1, 2),
			TextStyle: lipgloss.NewStyle(),
		},
	}
}
