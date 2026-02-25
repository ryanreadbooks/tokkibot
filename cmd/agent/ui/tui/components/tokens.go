package components

import (
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ryanreadbooks/tokkibot/cmd/agent/ui/tui/styles"
)

// TokensComponent displays token count
type TokensComponent struct {
	viewport viewport.Model
	theme    *styles.Theme
	count    int64
}

// NewTokensComponent creates a new tokens component
func NewTokensComponent(theme *styles.Theme) *TokensComponent {
	vp := viewport.New(80, 1)
	vp.MouseWheelEnabled = false
	vp.KeyMap = viewport.KeyMap{}

	return &TokensComponent{
		viewport: vp,
		theme:    theme,
		count:    0,
	}
}

// Init initializes the component
func (c *TokensComponent) Init() tea.Cmd {
	return nil
}

// Update handles component updates
func (c *TokensComponent) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	c.viewport, cmd = c.viewport.Update(msg)
	return cmd
}

// View renders the component
func (c *TokensComponent) View() string {
	return c.viewport.View()
}

// SetCount updates the token count
func (c *TokensComponent) SetCount(count int64) {
	if c.count != count {
		c.count = count
		c.refresh()
	}
}

// GetCount returns the current token count
func (c *TokensComponent) GetCount() int64 {
	return c.count
}

// SetWidth sets the component width
func (c *TokensComponent) SetWidth(width int) {
	c.viewport.Width = width
}

// refresh updates the viewport content
func (c *TokensComponent) refresh() {
	text := c.theme.Tokens.TextStyle.Render(fmt.Sprintf("Tokens: %d", c.count))
	c.viewport.SetContent(text)
}
