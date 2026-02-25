package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ryanreadbooks/tokkibot/agent"
)

// SpinnerModel is a simple model showing a spinner while agent is thinking
type SpinnerModel struct {
	spinner spinner.Model
	done    bool
	answer  string
}

// answerMsg carries the agent's answer
type answerMsg string

// NewSpinnerModel creates a new spinner model
func NewSpinnerModel() SpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#2ECC71"))

	return SpinnerModel{
		spinner: s,
	}
}

func (m SpinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m SpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		return m, nil

	case answerMsg:
		m.done = true
		m.answer = string(msg)
		return m, tea.Quit

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m SpinnerModel) View() string {
	if m.done {
		return ""
	}
	return fmt.Sprintf("%s Agent thinking...", m.spinner.View())
}

// RunWithSpinner runs agent with a spinner animation
func RunWithSpinner(ctx context.Context, ag *agent.Agent, channel, chatID, message string) error {
	model := NewSpinnerModel()
	program := tea.NewProgram(model)

	// Run agent in background
	go func() {
		answer := ag.Ask(ctx, &agent.UserMessage{
			Channel: channel,
			ChatId:  chatID,
			Created: 0, // Will be set by agent
			Content: message,
		})
		program.Send(answerMsg(answer))
	}()

	finalModel, err := program.Run()
	if err != nil {
		return err
	}

	// Print answer
	if m, ok := finalModel.(SpinnerModel); ok {
		fmt.Println(m.answer)
	}

	return nil
}
