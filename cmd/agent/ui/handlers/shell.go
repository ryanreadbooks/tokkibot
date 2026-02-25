package handlers

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ryanreadbooks/tokkibot/cmd/agent/ui/types"
)

// ShellConfirmHandler handles shell command confirmations
type ShellConfirmHandler struct {
	program *tea.Program
}

// NewShellConfirmHandler creates a new shell confirm handler
func NewShellConfirmHandler(program *tea.Program) *ShellConfirmHandler {
	return &ShellConfirmHandler{
		program: program,
	}
}

// ConfirmCommand implements the ShellConfirmer interface
func (h *ShellConfirmHandler) ConfirmCommand(command string) (bool, error) {
	respCh := make(chan bool, 1)

	// Send confirmation request to UI
	h.program.Send(types.ShellConfirmRequest{
		Command: command,
		RespCh:  respCh,
	})

	// Wait for user response
	confirmed := <-respCh
	return confirmed, nil
}
