package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ryanreadbooks/tokkibot/agent"
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

	// State
	width    int
	height   int
	curRound int
	err      error
}

// New creates a new TUI model
func New(
	ctx context.Context,
	ag *agent.Agent,
	channel string,
	chatID string,
	initMessages []types.Message,
) Model {
	theme := styles.DefaultTheme()
	handler := handlers.NewAgentHandler(ag, channel, chatID)

	// Create components
	chat := components.NewChatComponent(theme)
	chat.LoadMessages(initMessages)

	input := components.NewInputComponent(inputHeight)
	toolCall := components.NewToolCallComponent(theme)
	tokens := components.NewTokensComponent(theme)
	tokens.SetCount(handler.GetTokens())

	confirm := components.NewConfirmDialog(theme)

	return Model{
		ctx:      ctx,
		handler:  handler,
		theme:    theme,
		chat:     chat,
		input:    input,
		toolCall: toolCall,
		tokens:   tokens,
		confirm:  confirm,
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
	)
}
