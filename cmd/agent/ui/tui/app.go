package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ryanreadbooks/tokkibot/agent"
	"github.com/ryanreadbooks/tokkibot/agent/tools"
	"github.com/ryanreadbooks/tokkibot/cmd/agent/ui/handlers"
	"github.com/ryanreadbooks/tokkibot/cmd/agent/ui/types"
)

// Run starts the TUI application
func Run(
	ctx context.Context,
	ag *agent.Agent,
	channel string,
	chatID string,
) error {
	// Initialize session
	handler := handlers.NewAgentHandler(ag, channel, chatID)
	if err := handler.InitSession(); err != nil {
		return fmt.Errorf("failed to init session: %w", err)
	}

	// Load history
	history, err := handler.LoadHistory()
	if err != nil {
		// Ignore not found errors
		history = []types.Message{}
	}

	// Create model
	model := New(ctx, ag, channel, chatID, history)

	// Create program
	program := tea.NewProgram(&model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Set program reference for callbacks
	model.SetProgram(program)

	// Inject shell confirmer
	shellHandler := model.GetShellConfirmHandler()
	ctx = tools.WithShellConfirmer(ctx, shellHandler)
	model.ctx = ctx

	// Run
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
