package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ryanreadbooks/tokkibot/agent"
	cliadapter "github.com/ryanreadbooks/tokkibot/channel/adapter/cli"
	"github.com/ryanreadbooks/tokkibot/cmd/agent/ui/handlers"
	"github.com/ryanreadbooks/tokkibot/cmd/agent/ui/types"
)

// Run starts the TUI application with CLI adapter (via gateway)
func Run(
	ctx context.Context,
	ag *agent.Agent,
	adapter *cliadapter.CLIAdapter,
) error {
	handler := handlers.NewAgentHandler(ag, adapter)
	if err := handler.InitSession(); err != nil {
		return fmt.Errorf("failed to init session: %w", err)
	}

	history, err := handler.LoadHistory()
	if err != nil {
		history = []types.Message{}
	}

	model := New(ctx, handler, history)

	program := tea.NewProgram(&model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	model.SetProgram(program)

	if _, err := program.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
