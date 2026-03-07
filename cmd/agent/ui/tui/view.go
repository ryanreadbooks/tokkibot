package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// View renders the entire UI
func (m Model) View() string {
	// Show confirmation dialog if visible
	if m.confirm.IsVisible() {
		content := fmt.Sprintf("%s\n%s",
			m.chat.View(),
			m.confirm.View())
		return lipgloss.PlaceVertical(m.height, lipgloss.Bottom, content)
	}

	// Build status line
	statusLine := m.tokens.View()
	if m.processing {
		statusLine = fmt.Sprintf("%s Processing...", m.spinner.View())
	}

	// Normal view - tool calls are now part of chat
	content := fmt.Sprintf("%s\n%s\n%s",
		m.chat.View(),
		m.input.View(),
		statusLine)

	// Place content at bottom to avoid empty space at bottom
	return lipgloss.PlaceVertical(m.height, lipgloss.Bottom, content)
}
