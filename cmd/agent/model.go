package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ryanreadbooks/tokkibot/agent"
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
		Bold(true).BorderLeft(true)
	userMsgStyle = lipgloss.NewStyle().Foreground(
		userStyleColor).AlignHorizontal(lipgloss.Left)

	assistantStyle = lipgloss.NewStyle().Foreground(
		assistantStyleColor).Bold(true).BorderLeft(true)
	assistantMsgStyle = lipgloss.NewStyle().Foreground(
		assistantStyleColor).AlignHorizontal(lipgloss.Left)

	assistantThinkingStyle = lipgloss.NewStyle().Foreground(
		assistantThinkingStyleColor).Bold(true).BorderLeft(true)
	assistantThinkingMsgStyle = lipgloss.NewStyle().Foreground(
		assistantThinkingStyleColor).AlignHorizontal(lipgloss.Left)

	toolCallStyle = lipgloss.NewStyle().Foreground(
		lipgloss.Color("#e6ca3d"),
	).Italic(true).Bold(true)

	toolCallArgsStyle = lipgloss.NewStyle().Foreground(
		lipgloss.Color("#5e5e5e"),
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
	msgVp.KeyMap = viewport.KeyMap{}
	tcVp := viewport.New(80, 5)

	tokensVp := viewport.New(80, 1)
	tokensVp.MouseWheelEnabled = true
	tokensVp.KeyMap = viewport.KeyMap{}
	tokens := ag.GetCurrentContextTokens(chmodel.ChannelCLI.String(), agentChatId)

	return agentModel{
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

	m.curTokensViewport.SetContent(fmt.Sprintf("Tokens: %d", m.curTokens))
	m.curTokensViewport.GotoBottom()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.msgViewport.Width = msg.Width
		if m.curToolCall.name != "" {
			m.toolCallViewport.Height = 3
			m.msgViewport.Height = msg.Height - textAreaHeight*3 - m.toolCallViewport.Height
		} else {
			m.msgViewport.Height = msg.Height - textAreaHeight*3
		}
		m.inputTextarea.SetWidth(msg.Width)
		m.updateMsgViewport()

	case tea.KeyMsg:
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

			// invoke agent loop
			stream := m.ag.AskStream(m.ctx, &agent.UserMessage{
				Channel: chmodel.ChannelCLI.String(),
				ChatId:  agentChatId,
				Created: time.Now().Unix(),
				Content: userInput,
			})
			m.consumeChans(stream)
			m.inputTextarea.Reset()
			m.updateMsgViewport()

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
		m.toolCallViewport.GotoBottom()
		m.msgViewport.GotoBottom()

	case clearRoundMsg:
		m.curRound = int(msg)

	case errMsg:
		m.err = msg
		return m, nil
	}

	return m, tea.Batch(taCmd, vpCmd, spCmd, tokensCmd)
}

func renderUserMsg(content string) string {
	return userStyle.Render("You: ") + userMsgStyle.Render(content) + "\n"
}

func renderAssistantThinkingMsg(content string) string {
	return assistantThinkingStyle.Render("Thinking: ") + assistantThinkingMsgStyle.Render(content) + "\n"
}

func renderAssistantMsg(content string) string {
	return assistantStyle.Render("Assistant: ") + assistantMsgStyle.Render(content) + "\n"
}

func (m agentModel) renderUiMsgs() string {
	if len(m.msgs) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, msg := range m.msgs {
		switch msg.role {
		case roleUser:
			sb.WriteString(renderUserMsg(msg.content.content))
		case roleAssistant:
			// add content
			if msg.content.reasoningContent != "" {
				sb.WriteString(renderAssistantThinkingMsg(msg.content.reasoningContent))
			}
			if msg.content.content != "" {
				sb.WriteString(renderAssistantMsg(msg.content.content))
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

func (m *agentModel) renderToolCall() string {
	if m.curToolCall.name != "" {
		return toolCallStyle.Render("Tool calling "+m.curToolCall.name+" ") +
			m.toolCallingSpinner.View() + "\n" +
			toolCallArgsStyle.Render(m.curToolCall.arguments) + "\n"
	}

	return ""
}

func (m agentModel) View() string {
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

		// finish and clear round
		m.pg.Send(clearRoundMsg(-1))
	}()

	// tool calling
	go func() {
		for tc := range stream.ToolCall {
			m.pg.Send(toolCallMsg{
				round:     tc.Round,
				name:      tc.Name,
				arguments: tc.Arguments,
			})
		}

		// before exit we need to dismiss tool call display
		m.pg.Send(toolCallMsg{})
	}()

	return m
}

func (m *agentModel) setPg(pg *tea.Program) {
	m.pg = pg
}
