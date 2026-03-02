package tui

import "fmt"

// View renders the entire UI
func (m Model) View() string {
	// Show confirmation dialog if visible
	if m.confirm.IsVisible() {
		return fmt.Sprintf("%s\n%s",
			m.chat.View(),
			m.confirm.View())
	}

	// Build status line
	statusLine := m.tokens.View()
	if m.processing {
		statusLine = fmt.Sprintf("%s Processing...", m.spinner.View())
	}

	// Normal view - tool calls are now part of chat
	return fmt.Sprintf("%s\n%s\n%s",
		m.chat.View(),
		m.input.View(),
		statusLine)
}
