package components

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ryanreadbooks/tokkibot/cmd/agent/ui/tui/styles"
	"github.com/ryanreadbooks/tokkibot/cmd/agent/ui/types"
)

// ToolCallComponent displays current tool call information
type ToolCallComponent struct {
	viewport     viewport.Model
	spinner      spinner.Model
	theme        *styles.Theme
	name         string
	argsRaw      string // raw JSON arguments (streaming)
	argsComplete bool   // whether args are complete
	visible      bool
}

// NewToolCallComponent creates a new tool call component
func NewToolCallComponent(theme *styles.Theme) *ToolCallComponent {
	vp := viewport.New(80, 8)
	vp.MouseWheelEnabled = true
	vp.KeyMap = viewport.DefaultKeyMap()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = sp.Style.Foreground(styles.ColorSpinner)

	return &ToolCallComponent{
		viewport: vp,
		spinner:  sp,
		theme:    theme,
		visible:  false,
	}
}

// Init initializes the component
func (c *ToolCallComponent) Init() tea.Cmd {
	return c.spinner.Tick
}

// Update handles component updates
func (c *ToolCallComponent) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	var vpCmd tea.Cmd
	c.viewport, vpCmd = c.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	var spCmd tea.Cmd
	c.spinner, spCmd = c.spinner.Update(msg)
	cmds = append(cmds, spCmd)

	return tea.Batch(cmds...)
}

// View renders the component
func (c *ToolCallComponent) View() string {
	if !c.visible {
		return ""
	}
	return c.viewport.View()
}

// Show displays a tool call
func (c *ToolCallComponent) Show(name, args string) {
	if name == "" {
		// Invalid call, ignore
		return
	}

	// Check if this is a new tool call (different name)
	if c.name != name {
		// New tool call - reset state
		c.name = name
		c.argsRaw = ""
		c.argsComplete = false
	}

	c.visible = true

	// Update arguments
	if args != "" {
		// Complete arguments received
		c.argsRaw = args
		c.argsComplete = true
	}
	// If args == "", keep argsComplete = false (collecting state)

	c.refresh()
}

// Hide hides the tool call display
func (c *ToolCallComponent) Hide() {
	c.visible = false
	c.name = ""
	c.argsRaw = ""
	c.argsComplete = false
}

// IsVisible returns whether the component is visible
func (c *ToolCallComponent) IsVisible() bool {
	return c.visible
}

// SetSize sets the component size
func (c *ToolCallComponent) SetSize(width, height int) {
	c.viewport.Width = width
	c.viewport.Height = height
}

// refresh updates the viewport content
func (c *ToolCallComponent) refresh() {
	if !c.visible {
		c.viewport.SetContent("")
		return
	}

	if c.name == "" {
		c.viewport.SetContent("(waiting for tool call...)")
		return
	}

	// Format tool call display
	var argsDisplay string
	if c.argsComplete {
		// Show formatted arguments
		argsDisplay = types.FormatToolCallArgs(c.name, c.argsRaw, 100)
	} else {
		// Show streaming indicator
		argsDisplay = "collecting arguments " + c.spinner.View()
	}

	content := c.theme.ToolCall.NameStyle.Render("🔧 "+c.name) + " " +
		c.spinner.View() + "\n" +
		c.theme.ToolCall.ArgsStyle.Render(argsDisplay)

	c.viewport.SetContent(content)
	c.viewport.GotoBottom()
}
