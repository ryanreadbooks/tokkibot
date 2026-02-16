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

func giveupChannel[T any](c <-chan T) {
	go func() {
		for range c {
		}
	}()
}

var (
	userStyleColor              = lipgloss.Color("#937dd8")
	assistantStyleColor         = lipgloss.Color("#0f8b56")
	assistantThinkingStyleColor = lipgloss.Color("#5e5e5e")

	userStyle    = lipgloss.NewStyle().Foreground(userStyleColor).Bold(true).BorderLeft(true)
	userMsgStyle = lipgloss.NewStyle().Foreground(userStyleColor).AlignHorizontal(lipgloss.Left)

	assistantStyle    = lipgloss.NewStyle().Foreground(assistantStyleColor).Bold(true).BorderLeft(true)
	assistantMsgStyle = lipgloss.NewStyle().Foreground(assistantStyleColor).AlignHorizontal(lipgloss.Left)

	assistantThinkingStyle    = lipgloss.NewStyle().Foreground(assistantThinkingStyleColor).Bold(true).BorderLeft(true)
	assistantThinkingMsgStyle = lipgloss.NewStyle().Foreground(assistantThinkingStyleColor).AlignHorizontal(lipgloss.Left)
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

		width    int // terminal width
		textarea textarea.Model
		viewport viewport.Model
		spinner  spinner.Model
		err      error
	}

	contentMsg struct {
		content          string
		reasoningContent string
	}
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

	vp := viewport.New(80, 20)

	return agentModel{
		ctx:      ctx,
		ag:       ag,
		textarea: ta,
		viewport: vp,
		spinner:  sp,
		msgs:     initMsgs,
	}
}

func (m agentModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink)
}

func (m agentModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		taCmd tea.Cmd
		spCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textarea, taCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)
	m.spinner, spCmd = m.spinner.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - textAreaHeight*3
		m.textarea.SetWidth(msg.Width)
		m.updateViewport()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			userInput := m.textarea.Value()
			userInput = strings.TrimSpace(userInput)
			if userInput == "" {
				return m, tea.Batch(taCmd, spCmd)
			}

			// add msg
			m.msgs = append(m.msgs, uiMsg{role: roleUser, content: uiMsgContent{content: userInput}})
			// add assistant placeholder
			m.msgs = append(m.msgs, uiMsg{role: roleAssistant})

			// invoke agent loop
			stream := m.ag.AskStream(m.ctx, &chmodel.IncomingMessage{
				Channel: chmodel.ChannelCLI,
				ChatId:  agentChatId,
				Created: time.Now().Unix(),
				Content: userInput,
			})
			m.consumeChans(stream)
			m.textarea.Reset()
			m.updateViewport()

			return m, tea.Batch(taCmd, vpCmd, spCmd)
		}

	case contentMsg:
		if len(m.msgs) != 0 {
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

		m.updateViewport()

		// add to the last msg
		return m, tea.Batch(taCmd, vpCmd, spCmd)

	case errMsg:
		m.err = msg
		return m, nil
	}

	return m, tea.Batch(taCmd, vpCmd, spCmd)
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

func (m *agentModel) updateViewport() {
	content := m.renderUiMsgs()
	m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width).Render(content))
	m.viewport.GotoBottom()
}

func (m agentModel) View() string {
	return fmt.Sprintf("%s\n\n%s", m.viewport.View(), m.textarea.View())
}

func (m agentModel) consumeChans(stream *agent.AskStreamResult) tea.Model {
	go func() {
		for c := range stream.Content {
			m.pg.Send(contentMsg{
				content:          c.Content,
				reasoningContent: c.ReasoningContent,
			})
		}
	}()

	giveupChannel(stream.ToolCall)

	return m
}

func (m *agentModel) setPg(pg *tea.Program) {
	m.pg = pg
}
