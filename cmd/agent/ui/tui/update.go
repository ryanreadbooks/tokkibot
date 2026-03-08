package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ryanreadbooks/tokkibot/cmd/agent/ui/types"
)

// Update handles all model updates
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle specific messages first
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg), nil

	case tea.KeyMsg:
		// For key messages, handle special keys first before passing to input
		newModel, cmd := m.handleKeyPress(msg)
		if cmd != nil {
			return newModel, cmd
		}
		// If not handled, pass to input component
		inputCmd := m.input.Update(msg)
		return newModel, inputCmd

	case tea.MouseMsg:
		// Pass mouse events to chat for scrolling
		chatCmd := m.chat.Update(msg)
		return m, chatCmd

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case ContentMsg:
		return m.handleContent(msg), nil

	case ToolCallMsg:
		return m.handleToolCall(msg), nil

	case ClearRoundMsg:
		return m.handleClearRound(msg)

	case TokensUpdateMsg:
		return m.handleTokensUpdate(msg), nil

	case ToolConfirmMsg:
		return m.handleToolConfirm(msg), nil

	case ErrorMsg:
		m.err = msg
		return m, nil
	}

	// For other messages (like blink), pass to input
	inputCmd := m.input.Update(msg)
	if inputCmd != nil {
		cmds = append(cmds, inputCmd)
	}

	return m, tea.Batch(cmds...)
}

// handleWindowSize handles window resize events
func (m Model) handleWindowSize(msg tea.WindowSizeMsg) Model {
	m.width = msg.Width
	m.height = msg.Height

	// Calculate heights:
	// - input area: inputHeight (2 lines)
	// - tokens line: 1 line
	// - newlines in View: 2 lines (\n before input, \n before tokens)
	tokensHeight := 1
	newlinesCount := 2
	reservedHeight := inputHeight + tokensHeight + newlinesCount
	chatHeight := msg.Height - reservedHeight
	if chatHeight < 5 {
		chatHeight = 5
	}

	// Update component sizes
	m.chat.SetSize(msg.Width, chatHeight)
	m.input.SetWidth(msg.Width)
	m.tokens.SetWidth(msg.Width)
	m.confirm.SetWidth(msg.Width)

	return m
}

// handleKeyPress handles keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (Model, tea.Cmd) {
	// Handle shell confirmation if visible
	if m.confirm.IsVisible() {
		switch msg.Type {
		case tea.KeyCtrlC:
			m.confirm.Reject()
			return m, nil
		case tea.KeyEnter:
			m.confirm.Accept()
			return m, nil
		case tea.KeyEsc:
			m.confirm.Reject()
			return m, nil
		}
		// Ignore other keys during confirmation
		return m, nil
	}

	// Handle normal key presses
	switch msg.Type {
	case tea.KeyCtrlC:
		if m.processing && m.cancelFn != nil {
			m.cancelFn()
			return m, nil
		}
		return m, tea.Quit

	case tea.KeyEnter:
		userInput := m.input.Value()
		if userInput == "" {
			return m, nil
		}

		// Ignore if already processing
		if m.processing {
			return m, nil
		}

		// Set processing state
		m.processing = true
		reqCtx, cancelFn := context.WithCancel(m.ctx)
		m.cancelFn = cancelFn

		// Add user message
		m.chat.AddMessage(types.Message{
			Role:      types.RoleUser,
			Content:   userInput,
			Timestamp: time.Now(),
		})

		// Add assistant placeholder
		m.chat.AddMessage(types.Message{
			Role:      types.RoleAssistant,
			Timestamp: time.Now(),
		})

		// Send to agent via handler
		stream := m.handler.SendMessage(reqCtx, userInput)

		// Capture program reference for goroutines
		program := m.program

		// Consume content stream
		go func() {
			for c := range stream.Content {
				program.Send(ContentMsg{
					Round:            c.Round,
					Content:          c.Content,
					ReasoningContent: c.ReasoningContent,
				})
			}
			// Signal round completion
			program.Send(ClearRoundMsg(-1))
			program.Send(TokensUpdateMsg{})
		}()

		// Consume tool call stream
		go func() {
			for tc := range stream.ToolCall {
				program.Send(ToolCallMsg{
					Round:     tc.Round,
					Name:      tc.Name,
					Arguments: tc.Arguments,
				})
			}
			// Signal completion
			program.Send(ToolCallMsg{})
		}()

		// Consume confirmation stream
		go func() {
			for event := range stream.Confirm {
				if event != nil {
					program.Send(ToolConfirmMsg{
						Request: event.Request,
						RespCh:  event.RespCh,
					})
				}
			}
		}()

		// Reset input (this will refocus automatically)
		m.input.Reset()

		// Return with input initialization command to keep cursor blinking
		return m, m.input.Init()
	}

	return m, nil
}

// handleContent handles streaming content from assistant
func (m Model) handleContent(msg ContentMsg) Model {
	// Handle new round
	if m.curRound != msg.Round && msg.Round != -1 {
		m.chat.AddMessage(types.Message{
			Role:      types.RoleAssistant,
			Timestamp: time.Now(),
		})
		m.curRound = msg.Round
	}

	// Update last message
	m.chat.UpdateLastMessage(msg.Content, msg.ReasoningContent)

	return m
}

// handleToolCall handles tool call events
func (m Model) handleToolCall(msg ToolCallMsg) Model {
	if msg.Name == "" {
		// Tool call finished, mark last tool message as complete
		// Find and update the last tool call message
		return m
	}

	// Check if this is initial notification (empty args) or complete (with args)
	if msg.Arguments == "" {
		// Initial notification - add in-progress tool call message
		m.chat.AddMessage(types.Message{
			Role:         types.RoleToolCall,
			ToolName:     msg.Name,
			ToolComplete: false,
			Timestamp:    time.Now(),
		})
	} else {
		// Complete arguments - update last tool call message
		m.chat.UpdateLastToolCall(msg.Name, msg.Arguments)
	}

	return m
}

// handleClearRound handles round clear events
func (m Model) handleClearRound(msg ClearRoundMsg) (Model, tea.Cmd) {
	m.curRound = int(msg)
	m.processing = false
	m.cancelFn = nil

	// Return a command to update tokens (instead of calling Send directly)
	return m, func() tea.Msg {
		return TokensUpdateMsg{}
	}
}

// handleTokensUpdate handles token count updates
func (m Model) handleTokensUpdate(msg TokensUpdateMsg) Model {
	newTokens := m.handler.GetTokens()
	m.tokens.SetCount(newTokens)
	return m
}

// handleToolConfirm handles tool confirmation requests
func (m Model) handleToolConfirm(msg ToolConfirmMsg) Model {
	m.confirm.Show(msg.Request, msg.RespCh)
	return m
}

