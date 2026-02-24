package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ryanreadbooks/tokkibot/agent"
	"github.com/ryanreadbooks/tokkibot/agent/tools"
	chmodel "github.com/ryanreadbooks/tokkibot/channel/model"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	roleUser      = "user"
	roleAssistant = "assistant"
)

var (
	userStyleColor              = lipgloss.Color("#937dd8")
	assistantStyleColor         = lipgloss.Color("#0f8b56")
	assistantThinkingStyleColor = lipgloss.Color("#5e5e5e")

	userStyle = lipgloss.NewStyle().Foreground(
		userStyleColor).
		Bold(true)
	userMsgStyle = lipgloss.NewStyle().Foreground(
		userStyleColor).AlignHorizontal(lipgloss.Left)
	userBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true, true, true, true).
		BorderForeground(lipgloss.Color("#937dd8")).
		Padding(0, 1).
		MarginBottom(1)

	assistantStyle = lipgloss.NewStyle().Foreground(
		assistantStyleColor).Bold(true)
	assistantMsgStyle = lipgloss.NewStyle().Foreground(
		assistantStyleColor).AlignHorizontal(lipgloss.Left)
	assistantBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true, true, true, true).
		BorderForeground(lipgloss.Color("#0f8b56")).
		Padding(0, 1).
		MarginBottom(1)

	assistantThinkingStyle = lipgloss.NewStyle().Foreground(
		assistantThinkingStyleColor).Bold(true)
	assistantThinkingMsgStyle = lipgloss.NewStyle().Foreground(
		assistantThinkingStyleColor).AlignHorizontal(lipgloss.Left)

	toolCallStyle = lipgloss.NewStyle().Foreground(
		lipgloss.Color("#e6ca3d"),
	).Italic(true).Bold(true)

	toolCallArgsStyle = lipgloss.NewStyle().Foreground(
		lipgloss.Color("#5e5e5e"),
	)

	tokensStyle = lipgloss.NewStyle().Foreground(
		lipgloss.Color("#808080"),
	)
)

type (
	errMsg error

	uiMsgContent struct {
		content          string
		reasoningContent string
	}

	uiMsg struct {
		role    string       // user or assistant
		content uiMsgContent // for user and assistant
	}

	shellConfirmRequest struct {
		command string
		respCh  chan bool
	}

	agentModel struct {
		pg  *tea.Program
		ctx context.Context
		ag  *agent.Agent

		// all msgs
		msgs []uiMsg

		width         int // terminal width
		inputTextarea textarea.Model
		msgViewport   viewport.Model

		toolCallViewport   viewport.Model
		toolCallingSpinner spinner.Model
		curToolCall        toolCallMsg

		err error

		curRound int

		curTokensViewport viewport.Model
		curTokens         int64
		
		// Shell confirmation
		shellConfirmPending *shellConfirmRequest
	}

	contentMsg struct {
		round            int
		content          string
		reasoningContent string
	}

	toolCallMsg struct {
		round     int
		name      string
		arguments string
	}

	clearRoundMsg int

	updateTokensMsg struct{}
	
	shellConfirmMsg shellConfirmRequest
)

const (
	textAreaHeight = 2
)

func initAgentModel(
	ctx context.Context,
	ag *agent.Agent,
	initMsgs []uiMsg,
) agentModel {
	ta := textarea.New()
	ta.Placeholder = "What can I do for you?"
	ta.Focus()
	ta.Prompt = "┃ "
	ta.SetHeight(textAreaHeight)
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#2ECC71"))

	msgVp := viewport.New(80, 20)
	msgVp.MouseWheelEnabled = true
	msgVp.KeyMap = viewport.DefaultKeyMap()
	
	tcVp := viewport.New(80, 8)
	tcVp.MouseWheelEnabled = true
	tcVp.KeyMap = viewport.DefaultKeyMap()

	tokensVp := viewport.New(80, 1)
	tokensVp.MouseWheelEnabled = false
	tokensVp.KeyMap = viewport.KeyMap{}
	tokens := ag.GetCurrentContextTokens(chmodel.ChannelCLI.String(), agentChatId)
	tokensVp.SetContent(tokensStyle.Render(fmt.Sprintf("Tokens: %d", tokens)))

	mod := agentModel{
		ctx:                ctx,
		ag:                 ag,
		inputTextarea:      ta,
		msgViewport:        msgVp,
		toolCallViewport:   tcVp,
		toolCallingSpinner: sp,
		msgs:               initMsgs,
		curRound:           -1,
		curTokensViewport:  tokensVp,
		curTokens:          tokens,
	}
	
	return mod
}

// ConfirmCommand implements ShellConfirmer interface
func (m *agentModel) ConfirmCommand(command string) (bool, error) {
	respCh := make(chan bool, 1)
	
	// Send confirmation request to UI
	m.pg.Send(shellConfirmMsg{
		command: command,
		respCh:  respCh,
	})
	
	// Wait for user response
	confirmed := <-respCh
	return confirmed, nil
}

func (m agentModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.toolCallingSpinner.Tick)
}

func (m agentModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		taCmd     tea.Cmd
		spCmd     tea.Cmd
		vpCmd     tea.Cmd
		tokensCmd tea.Cmd
	)

	m.inputTextarea, taCmd = m.inputTextarea.Update(msg)
	m.msgViewport, vpCmd = m.msgViewport.Update(msg)
	m.toolCallingSpinner, spCmd = m.toolCallingSpinner.Update(msg)
	m.curTokensViewport, tokensCmd = m.curTokensViewport.Update(msg)
	
	var tcVpCmd tea.Cmd
	m.toolCallViewport, tcVpCmd = m.toolCallViewport.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.msgViewport.Width = msg.Width
		m.toolCallViewport.Width = msg.Width
		if m.curToolCall.name != "" {
			m.toolCallViewport.Height = 8
			m.msgViewport.Height = msg.Height - textAreaHeight*3 - m.toolCallViewport.Height
		} else {
			m.msgViewport.Height = msg.Height - textAreaHeight*3
		}
		m.inputTextarea.SetWidth(msg.Width)
		m.updateMsgViewport()

	case tea.KeyMsg:
		// Handle shell confirmation if pending
		if m.shellConfirmPending != nil {
			switch msg.Type {
			case tea.KeyCtrlC:
				// Cancel confirmation
				m.shellConfirmPending.respCh <- false
				close(m.shellConfirmPending.respCh)
				m.shellConfirmPending = nil
				return m, nil
			case tea.KeyEnter:
				// Accept confirmation
				m.shellConfirmPending.respCh <- true
				close(m.shellConfirmPending.respCh)
				m.shellConfirmPending = nil
				return m, nil
			case tea.KeyEsc:
				// Reject confirmation
				m.shellConfirmPending.respCh <- false
				close(m.shellConfirmPending.respCh)
				m.shellConfirmPending = nil
				return m, nil
			}
			// Ignore other keys during confirmation
			return m, nil
		}
		
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			userInput := m.inputTextarea.Value()
			userInput = strings.TrimSpace(userInput)
			if userInput == "" {
				return m, tea.Batch(taCmd, spCmd)
			}

			// add msg
			m.msgs = append(m.msgs, uiMsg{role: roleUser, content: uiMsgContent{content: userInput}})
			// add assistant placeholder
			m.msgs = append(m.msgs, uiMsg{role: roleAssistant})

			// Invoke agent loop
			stream := m.ag.AskStream(m.ctx, &agent.UserMessage{
				Channel: chmodel.ChannelCLI.String(),
				ChatId:  agentChatId,
				Created: time.Now().Unix(),
				Content: userInput,
			})
			m.consumeChans(stream)
			m.inputTextarea.Reset()
			m.updateMsgViewport()
			
			// Update tokens after user input
			go func() {
				time.Sleep(100 * time.Millisecond) // Wait for message to be appended
				m.pg.Send(updateTokensMsg{})
			}()

			return m, tea.Batch(taCmd, vpCmd, spCmd, tokensCmd)
		}

	case contentMsg:
		if len(m.msgs) != 0 {
			if m.curRound != msg.round && msg.round != -1 {
				// this is a new round
				m.msgs = append(m.msgs, uiMsg{role: roleAssistant})
				m.curRound = msg.round
			}

			idx := len(m.msgs) - 1
			if old := m.msgs[idx]; old.role == roleAssistant {
				m.msgs[idx] = uiMsg{
					role: roleAssistant,
					content: uiMsgContent{
						content:          old.content.content + msg.content,
						reasoningContent: old.content.reasoningContent + msg.reasoningContent,
					},
				}
			}
		}

		m.updateMsgViewport()

		// add to the last msg
		return m, tea.Batch(taCmd, vpCmd, spCmd, tokensCmd)

	case toolCallMsg:
		if msg.name == "" {
			m.curToolCall = msg
		} else {
			newToolCall := toolCallMsg{
				name:      msg.name,
				arguments: m.curToolCall.arguments + msg.arguments,
			}
			m.curToolCall = newToolCall
		}

		m.updateMsgViewport()
		m.updateToolCallViewport()
		m.msgViewport.GotoBottom()
		return m, tea.Batch(taCmd, vpCmd, spCmd, tokensCmd, tcVpCmd)

	case clearRoundMsg:
		m.curRound = int(msg)

	case updateTokensMsg:
		// Refresh token count from agent
		newTokens := m.ag.GetCurrentContextTokens(chmodel.ChannelCLI.String(), agentChatId)
		if newTokens != m.curTokens {
			m.curTokens = newTokens
			m.updateTokensViewport()
		}

	case shellConfirmMsg:
		// Store pending confirmation request
		m.shellConfirmPending = &shellConfirmRequest{
			command: msg.command,
			respCh:  msg.respCh,
		}
		return m, nil

	case errMsg:
		m.err = msg
		return m, nil
	}

	return m, tea.Batch(taCmd, vpCmd, spCmd, tokensCmd, tcVpCmd)
}

func (m agentModel) renderUserMsg(content string) string {
	header := userStyle.Render("You:")
	body := userMsgStyle.Render(content)
	boxWidth := m.msgViewport.Width - 4 // Account for padding and border
	if boxWidth < 20 {
		boxWidth = 20
	}
	box := userBoxStyle.Width(boxWidth).Render(header + "\n" + body)
	return box + "\n"
}

func (m agentModel) renderAssistantThinkingMsg(content string) string {
	header := assistantThinkingStyle.Render("Thinking:")
	body := assistantThinkingMsgStyle.Render(content)
	return header + "\n" + body + "\n"
}

func (m agentModel) renderAssistantMsg(content string) string {
	header := assistantStyle.Render("Assistant:")
	body := assistantMsgStyle.Render(content)
	boxWidth := m.msgViewport.Width - 4 // Account for padding and border
	if boxWidth < 20 {
		boxWidth = 20
	}
	box := assistantBoxStyle.Width(boxWidth).Render(header + "\n" + body)
	return box + "\n"
}

func (m agentModel) renderUiMsgs() string {
	if len(m.msgs) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, msg := range m.msgs {
		switch msg.role {
		case roleUser:
			sb.WriteString(m.renderUserMsg(msg.content.content))
		case roleAssistant:
			// add content
			if msg.content.reasoningContent != "" {
				sb.WriteString(m.renderAssistantThinkingMsg(msg.content.reasoningContent))
			}
			if msg.content.content != "" {
				sb.WriteString(m.renderAssistantMsg(msg.content.content))
			}
		}
	}

	return sb.String()
}

func (m *agentModel) updateMsgViewport() {
	content := m.renderUiMsgs()
	m.msgViewport.SetContent(lipgloss.NewStyle().Width(m.msgViewport.Width).Render(content))
	m.msgViewport.GotoBottom()
}

func (m *agentModel) updateTokensViewport() {
	tokenText := tokensStyle.Render(fmt.Sprintf("Tokens: %d", m.curTokens))
	m.curTokensViewport.SetContent(tokenText)
	m.curTokensViewport.GotoBottom()
}

func (m *agentModel) updateToolCallViewport() {
	if m.curToolCall.name != "" {
		content := toolCallStyle.Render("Tool calling "+m.curToolCall.name+" ") +
			m.toolCallingSpinner.View() + "\n" +
			toolCallArgsStyle.Render(m.curToolCall.arguments)
		m.toolCallViewport.SetContent(content)
		m.toolCallViewport.GotoBottom()
	}
}

func (m *agentModel) renderToolCall() string {
	if m.curToolCall.name != "" {
		return m.toolCallViewport.View()
	}
	return ""
}

func (m agentModel) View() string {
	// Show confirmation dialog if pending
	if m.shellConfirmPending != nil {
		confirmStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#ff6b6b")).
			Padding(1, 2).
			Width(m.msgViewport.Width - 4)
		
		confirmText := fmt.Sprintf("⚠️  Dangerous Command Detected\n\n%s\n\n[Enter] Accept  [Esc] Reject  [Ctrl+C] Cancel",
			m.shellConfirmPending.command)
		
		return fmt.Sprintf("%s\n\n%s",
			m.msgViewport.View(),
			confirmStyle.Render(confirmText))
	}
	
	if m.curToolCall.name != "" {
		return fmt.Sprintf("%s\n\n%s%s",
			m.msgViewport.View(),
			m.renderToolCall(),
			m.inputTextarea.View())
	}

	return fmt.Sprintf("%s\n\n%s\n%s",
		m.msgViewport.View(),
		m.inputTextarea.View(),
		m.curTokensViewport.View(),
	)
}

func (m *agentModel) consumeChans(stream *agent.AskStreamResult) tea.Model {
	go func() {
		for c := range stream.Content {
			m.pg.Send(contentMsg{
				round:            c.Round,
				content:          c.Content,
				reasoningContent: c.ReasoningContent,
			})
		}

		// Finish and clear round
		m.pg.Send(clearRoundMsg(-1))
		// Update tokens after conversation completes
		m.pg.Send(updateTokensMsg{})
	}()

	// Tool calling
	go func() {
		for tc := range stream.ToolCall {
			m.pg.Send(toolCallMsg{
				round:     tc.Round,
				name:      tc.Name,
				arguments: tc.Arguments,
			})
		}

		// Before exit we need to dismiss tool call display
		m.pg.Send(toolCallMsg{})
	}()

	return m
}

func (m *agentModel) setPg(pg *tea.Program) {
	m.pg = pg
}

func (m *agentModel) injectShellConfirmer() {
	// Inject confirmer into context
	m.ctx = tools.WithShellConfirmer(m.ctx, m)
}
