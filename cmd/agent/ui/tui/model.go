package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ryanreadbooks/tokkibot/cmd/agent/ui/handlers"
	"github.com/ryanreadbooks/tokkibot/cmd/agent/ui/tui/components"
	"github.com/ryanreadbooks/tokkibot/cmd/agent/ui/tui/styles"
	"github.com/ryanreadbooks/tokkibot/cmd/agent/ui/types"
)

const (
	inputHeight = 2
)

// Model is the main TUI model
type Model struct {
	// Context and dependencies
	ctx     context.Context
	handler *handlers.AgentHandler
	theme   *styles.Theme
	program *tea.Program

	// Components
	chat     *components.ChatComponent
	input    *components.InputComponent
	toolCall *components.ToolCallComponent
	tokens   *components.TokensComponent
	confirm  *components.ConfirmDialog
	spinner  spinner.Model

	// State
	width      int
	height     int
	curRound   int
	processing bool
	err        error
}

// New creates a new TUI model with handler
func New(
	ctx context.Context,
	handler *handlers.AgentHandler,
	initMessages []types.Message,
) Model {
	theme := styles.DefaultTheme()

	chat := components.NewChatComponent(theme)
	chat.LoadMessages(initMessages)

	input := components.NewInputComponent(inputHeight)
	toolCall := components.NewToolCallComponent(theme)
	tokens := components.NewTokensComponent(theme)
	tokens.SetCount(handler.GetTokens())

	confirm := components.NewConfirmDialog(theme)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#2ECC71"))

	return Model{
		ctx:      ctx,
		handler:  handler,
		theme:    theme,
		chat:     chat,
		input:    input,
		toolCall: toolCall,
		tokens:   tokens,
		confirm:  confirm,
		spinner:  s,
		width:    80,
		height:   24,
		curRound: -1,
	}
}

// SetProgram sets the tea program reference
func (m *Model) SetProgram(program *tea.Program) {
	m.program = program
}

// GetShellConfirmHandler returns a shell confirm handler
func (m *Model) GetShellConfirmHandler() *handlers.ShellConfirmHandler {
	return handlers.NewShellConfirmHandler(m.program)
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.input.Init(),
		m.toolCall.Init(),
		m.spinner.Tick,
	)
}
