package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ryanreadbooks/tokkibot/cmd/agent/ui/tui/styles"
	"github.com/ryanreadbooks/tokkibot/cmd/agent/ui/types"
)

// ChatComponent handles the chat message display area
type ChatComponent struct {
	viewport viewport.Model
	messages []types.Message
	theme    *styles.Theme
	width    int
	height   int
}

// NewChatComponent creates a new chat component
func NewChatComponent(theme *styles.Theme) *ChatComponent {
	vp := viewport.New(80, 20)
	vp.MouseWheelEnabled = true
	vp.KeyMap = viewport.DefaultKeyMap()

	return &ChatComponent{
		viewport: vp,
		messages: make([]types.Message, 0),
		theme:    theme,
		width:    80,
		height:   20,
	}
}

// Init initializes the component
func (c *ChatComponent) Init() tea.Cmd {
	return nil
}

// Update handles component updates
func (c *ChatComponent) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	c.viewport, cmd = c.viewport.Update(msg)
	return cmd
}

// View renders the component
func (c *ChatComponent) View() string {
	return c.viewport.View()
}

// AddMessage adds a new message to the chat
func (c *ChatComponent) AddMessage(msg types.Message) {
	c.messages = append(c.messages, msg)
	c.refresh()
}

// UpdateLastMessage updates the last message (for streaming)
func (c *ChatComponent) UpdateLastMessage(content, reasoningContent string) {
	if len(c.messages) == 0 {
		return
	}

	idx := len(c.messages) - 1
	if c.messages[idx].IsAssistant() {
		c.messages[idx].Content += content
		c.messages[idx].ReasoningContent += reasoningContent
		c.refresh()
	}
}

// UpdateLastToolCall updates the last tool call message with complete arguments
func (c *ChatComponent) UpdateLastToolCall(toolName, arguments string) {
	if len(c.messages) == 0 {
		return
	}

	// Find the last tool call message with matching name that's incomplete
	for i := len(c.messages) - 1; i >= 0; i-- {
		if c.messages[i].IsToolCall() && c.messages[i].ToolName == toolName && !c.messages[i].ToolComplete {
			c.messages[i].ToolArguments = arguments
			c.messages[i].ToolComplete = true
			c.refresh()
			return
		}
	}
}

// SetSize updates the component size
func (c *ChatComponent) SetSize(width, height int) {
	c.width = width
	c.height = height
	c.viewport.Width = width
	c.viewport.Height = height
	c.refresh()
}

// LoadMessages loads initial messages
func (c *ChatComponent) LoadMessages(messages []types.Message) {
	c.messages = messages
	c.refresh()
}

// refresh updates the viewport content
func (c *ChatComponent) refresh() {
	content := c.renderMessages()
	c.viewport.SetContent(lipgloss.NewStyle().Width(c.viewport.Width).Render(content))
	c.viewport.GotoBottom()
}

// renderMessages renders all messages
func (c *ChatComponent) renderMessages() string {
	if len(c.messages) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, msg := range c.messages {
		if msg.IsUser() {
			sb.WriteString(c.renderUserMessage(msg.Content))
		} else if msg.IsAssistant() {
			if msg.ReasoningContent != "" {
				sb.WriteString(c.renderThinkingMessage(msg.ReasoningContent))
			}
			if msg.Content != "" {
				sb.WriteString(c.renderAssistantMessage(msg.Content))
			}
		} else if msg.IsToolCall() {
			sb.WriteString(c.renderToolCallMessage(msg.ToolName, msg.ToolArguments))
		}
	}

	return sb.String()
}

// renderUserMessage renders a user message
func (c *ChatComponent) renderUserMessage(content string) string {
	header := c.theme.User.HeaderStyle.Render("You:")
	body := c.theme.User.BodyStyle.Render(content)
	
	boxWidth := c.viewport.Width - 4 // Account for padding and border
	if boxWidth < 20 {
		boxWidth = 20
	}
	
	box := c.theme.User.BoxStyle.Width(boxWidth).Render(header + "\n" + body)
	return box + "\n"
}

// renderAssistantMessage renders an assistant message
func (c *ChatComponent) renderAssistantMessage(content string) string {
	header := c.theme.Assistant.HeaderStyle.Render("Assistant:")
	body := c.theme.Assistant.BodyStyle.Render(content)
	
	boxWidth := c.viewport.Width - 4
	if boxWidth < 20 {
		boxWidth = 20
	}
	
	box := c.theme.Assistant.BoxStyle.Width(boxWidth).Render(header + "\n" + body)
	return box + "\n"
}

// renderThinkingMessage renders a thinking message
func (c *ChatComponent) renderThinkingMessage(content string) string {
	header := c.theme.Thinking.HeaderStyle.Render("Thinking:")
	body := c.theme.Thinking.BodyStyle.Render(content)
	return header + "\n" + body + "\n"
}

// renderToolCallMessage renders a tool call message (no box, similar to thinking)
func (c *ChatComponent) renderToolCallMessage(name, args string) string {
	header := c.theme.ToolCallMsg.HeaderStyle.Render("🔧 Tool: " + name)
	
	// Format arguments
	var argsDisplay string
	if args == "" {
		argsDisplay = "⟳ executing..."
	} else {
		argsDisplay = types.FormatToolCallArgs(name, args, 100)
	}
	
	body := c.theme.ToolCallMsg.BodyStyle.Render(argsDisplay)
	return header + "\n" + body + "\n"
}
