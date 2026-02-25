package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InputComponent handles user input
type InputComponent struct {
	textarea textarea.Model
	onSubmit func(string)
	height   int
}

// NewInputComponent creates a new input component
func NewInputComponent(height int) *InputComponent {
	ta := textarea.New()
	ta.Placeholder = "What can I do for you?"
	ta.Focus()
	ta.Prompt = "┃ "
	ta.SetHeight(height)
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()

	return &InputComponent{
		textarea: ta,
		height:   height,
	}
}

// Init initializes the component
func (c *InputComponent) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles component updates
func (c *InputComponent) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	c.textarea, cmd = c.textarea.Update(msg)
	return cmd
}

// View renders the component
func (c *InputComponent) View() string {
	return c.textarea.View()
}

// Value returns the current input value
func (c *InputComponent) Value() string {
	return strings.TrimSpace(c.textarea.Value())
}

// Reset clears the input and refocuses
func (c *InputComponent) Reset() {
	c.textarea.Reset()
	c.textarea.Focus()
}

// SetWidth sets the input width
func (c *InputComponent) SetWidth(width int) {
	c.textarea.SetWidth(width)
}

// SetOnSubmit sets the submit callback
func (c *InputComponent) SetOnSubmit(fn func(string)) {
	c.onSubmit = fn
}

// Focus focuses the input
func (c *InputComponent) Focus() {
	c.textarea.Focus()
}

// Blur blurs the input
func (c *InputComponent) Blur() {
	c.textarea.Blur()
}
