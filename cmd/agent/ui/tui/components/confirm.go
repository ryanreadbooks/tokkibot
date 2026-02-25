package components

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ryanreadbooks/tokkibot/cmd/agent/ui/tui/styles"
)

// ConfirmDialog displays a confirmation dialog
type ConfirmDialog struct {
	theme   *styles.Theme
	message string
	respCh  chan bool
	visible bool
	width   int
}

// NewConfirmDialog creates a new confirmation dialog
func NewConfirmDialog(theme *styles.Theme) *ConfirmDialog {
	return &ConfirmDialog{
		theme:   theme,
		visible: false,
		width:   80,
	}
}

// Init initializes the component
func (c *ConfirmDialog) Init() tea.Cmd {
	return nil
}

// Update handles component updates
func (c *ConfirmDialog) Update(msg tea.Msg) tea.Cmd {
	return nil
}

// View renders the component
func (c *ConfirmDialog) View() string {
	if !c.visible {
		return ""
	}

	boxWidth := c.width - 4
	if boxWidth < 40 {
		boxWidth = 40
	}

	confirmText := fmt.Sprintf("⚠️  Dangerous Command Detected\n\n%s\n\n[Enter] Accept  [Esc] Reject  [Ctrl+C] Cancel",
		c.message)

	return c.theme.Confirm.BoxStyle.Width(boxWidth).Render(confirmText)
}

// Show displays the confirmation dialog
func (c *ConfirmDialog) Show(message string, respCh chan bool) {
	c.message = message
	c.respCh = respCh
	c.visible = true
}

// Hide hides the confirmation dialog
func (c *ConfirmDialog) Hide() {
	c.visible = false
	c.message = ""
	c.respCh = nil
}

// IsVisible returns whether the dialog is visible
func (c *ConfirmDialog) IsVisible() bool {
	return c.visible
}

// Accept confirms the action
func (c *ConfirmDialog) Accept() {
	if c.respCh != nil {
		c.respCh <- true
		close(c.respCh)
	}
	c.Hide()
}

// Reject rejects the action
func (c *ConfirmDialog) Reject() {
	if c.respCh != nil {
		c.respCh <- false
		close(c.respCh)
	}
	c.Hide()
}

// SetWidth sets the dialog width
func (c *ConfirmDialog) SetWidth(width int) {
	c.width = width
}

// GetResponseChannel returns the response channel
func (c *ConfirmDialog) GetResponseChannel() chan bool {
	return c.respCh
}
