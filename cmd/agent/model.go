package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/ryanreadbooks/tokkibot/agent"
	"github.com/ryanreadbooks/tokkibot/channel"
	channelmodel "github.com/ryanreadbooks/tokkibot/channel/model"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	inputHeight   = 3 // height of textarea area including borders
	toolBoxHeight = 5 // height reserved for tool call box (border + 3 lines + spacing)
)

const (
	youPrefix   = "You: "
	agentPrefix = "Tokkibot: "
)

var (
	youStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
	agentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#2ECC71"))
)

type (
	errMsg              error
	agentStreamChunkMsg string   // streaming chunk from agent
	agentStreamDoneMsg  struct{} // signal that streaming is done

	// tool call thinking state
	toolCallMsg struct {
		Name        string
		ArgFragment string
		Done        bool // true when thinking is done
	}

	agentModel struct {
		ctx context.Context
		ag  *agent.Agent
		bus *channel.Bus

		messages        []string
		currentResponse *strings.Builder // accumulates streaming response (pointer to avoid copy issues)
		loading         bool

		// tool call display state (temporary, cleared when done)
		toolCallCh      <-chan toolCallMsg
		currentToolName string           // current tool name (always shown)
		currentToolArgs *strings.Builder // accumulated args (truncated for display)

		width    int // terminal width
		viewport viewport.Model
		textarea textarea.Model
		spinner  spinner.Model
		err      error
	}
)

func initAgentModel(ctx context.Context, ag *agent.Agent, bus *channel.Bus, history []string, toolCallCh <-chan toolCallMsg) agentModel {
	ta := textarea.New()
	ta.Placeholder = "What can I do for you?"
	ta.Focus()
	ta.Prompt = "┃ "
	ta.SetHeight(1) // single line input
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()

	vp := viewport.New(80, 20)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#2ECC71"))

	return agentModel{
		ctx:             ctx,
		ag:              ag,
		bus:             bus,
		messages:        history,
		currentResponse: &strings.Builder{},
		toolCallCh:      toolCallCh,
		currentToolArgs: &strings.Builder{},
		textarea:        ta,
		viewport:        vp,
		spinner:         sp,
	}
}

func (m agentModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.waitForStreamChunk(), m.waitForToolCall())
}

func (m agentModel) waitForToolCall() tea.Cmd {
	return func() tea.Msg {
		select {
		case <-m.ctx.Done():
			return nil
		case msg, ok := <-m.toolCallCh:
			if !ok {
				return nil
			}
			return msg
		}
	}
}

func (m agentModel) waitForStreamChunk() tea.Cmd {
	return func() tea.Msg {
		ch := m.bus.GetOutgoingChannel(channelmodel.ChannelCLI).Wait(m.ctx)
		select {
		case <-m.ctx.Done():
			return nil
		case msg, ok := <-ch:
			if !ok {
				return agentStreamDoneMsg{}
			}
			// CtrlStop signals end of stream for this response
			if msg.Ctrl == channelmodel.CtrlStop {
				return agentStreamDoneMsg{}
			}
			return agentStreamChunkMsg(msg.Content)
		}
	}
}

func (m agentModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		taCmd tea.Cmd
		vpCmd tea.Cmd
		spCmd tea.Cmd
	)

	m.textarea, taCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)
	m.spinner, spCmd = m.spinner.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.viewport.Width = msg.Width
		// Reserve space for tool box when loading
		extraHeight := 0
		if m.loading && m.currentToolName != "" {
			extraHeight = toolBoxHeight
		}
		m.viewport.Height = msg.Height - inputHeight - extraHeight
		m.textarea.SetWidth(msg.Width)
		m.updateViewportContent()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.loading {
				return m, tea.Batch(taCmd, vpCmd, spCmd)
			}
			userInput := m.textarea.Value()
			userInput = strings.TrimSpace(userInput)
			if userInput == "" {
				return m, tea.Batch(taCmd, vpCmd, spCmd)
			}
			m.messages = append(m.messages, youStyle.Render(youPrefix)+userInput)
			m.textarea.Reset()
			m.loading = true
			m.updateViewportContent()
			// send message to agent
			m.bus.GetIncomingChannel(channelmodel.ChannelCLI).Send(m.ctx, channelmodel.IncomingMessage{
				Channel: channelmodel.ChannelCLI,
				ChatId:  agentChatId,
				Content: userInput,
			})
			return m, tea.Batch(taCmd, vpCmd, m.spinner.Tick)
		}

	case agentStreamChunkMsg:
		// Append chunk to current response
		m.currentResponse.WriteString(string(msg))
		m.updateViewportContent()
		return m, tea.Batch(taCmd, vpCmd, spCmd, m.waitForStreamChunk())

	case agentStreamDoneMsg:
		// Streaming done, finalize the response
		if m.currentResponse.Len() > 0 {
			m.messages = append(m.messages, agentStyle.Render(agentPrefix)+m.currentResponse.String())
			m.currentResponse.Reset()
		}
		m.loading = false
		m.currentToolName = "" // clear tool call display
		m.currentToolArgs.Reset()
		m.updateViewportContent()
		return m, tea.Batch(taCmd, vpCmd, m.waitForStreamChunk())

	case toolCallMsg:
		if msg.Done {
			// Thinking done, clear tool call display
			m.currentToolName = ""
			m.currentToolArgs.Reset()
		} else {
			// New tool or same tool with more args
			if msg.Name != "" && msg.Name != m.currentToolName {
				// New tool call started
				m.currentToolName = msg.Name
				m.currentToolArgs.Reset()
			}
			// Accumulate argument fragments
			m.currentToolArgs.WriteString(msg.ArgFragment)
		}
		m.updateViewportContent()
		return m, tea.Batch(taCmd, vpCmd, spCmd, m.waitForToolCall())

	case spinner.TickMsg:
		if m.loading {
			m.updateViewportContent()
			return m, tea.Batch(taCmd, vpCmd, spCmd)
		}

	case errMsg:
		m.err = msg
		return m, nil
	}

	return m, tea.Batch(taCmd, vpCmd)
}

var (
	toolNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F39C12")).Bold(true)
	toolArgsStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
)

// renderToolCallBox renders a fixed-size box for tool call display
// The box has fixed dimensions - only the content inside changes
func renderToolCallBox(name string, args string, width int) string {
	// Box takes full width minus some margin
	boxWidth := width - 4
	if boxWidth < 20 {
		boxWidth = 20
	}
	// Inner content width = boxWidth - border(2) - padding(2)
	innerWidth := boxWidth - 4

	// Clean up args for display
	args = strings.ReplaceAll(args, "\n", " ")
	args = strings.Join(strings.Fields(args), " ")

	// Scroll args to show the tail if too long
	if len(args) > innerWidth {
		args = "..." + args[len(args)-innerWidth+3:]
	}

	// Pad args to fixed width so the line doesn't shrink
	if len(args) < innerWidth {
		args = args + strings.Repeat(" ", innerWidth-len(args))
	}

	header := toolNameStyle.Width(innerWidth).Render("⚙ " + name)
	argsLine := toolArgsStyle.Width(innerWidth).Render(args)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#F39C12")).
		Padding(0, 1).
		Width(boxWidth).
		Height(3) // Fixed height: 3 lines inside

	return boxStyle.Render(header + "\n" + argsLine)
}

// updateViewportContent refreshes viewport with current messages
func (m *agentModel) updateViewportContent() {
	var content string
	if len(m.messages) > 0 {
		content = strings.Join(m.messages, "\n")
	}

	// Show streaming response in progress (but NOT the tool box - that's rendered separately)
	if m.loading {
		var streamingLine string
		if m.currentResponse.Len() > 0 {
			// Show accumulated streaming content
			streamingLine = agentStyle.Render(agentPrefix) + m.currentResponse.String()
		} else {
			// Show spinner while waiting for first chunk
			streamingLine = agentStyle.Render(agentPrefix) + m.spinner.View()
		}

		if content != "" {
			content += "\n" + streamingLine
		} else {
			content = streamingLine
		}
	}

	m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width).Render(content))
	m.viewport.GotoBottom()
}

func (m agentModel) View() string {
	// Render tool call box between viewport and textarea (fixed position)
	toolBox := ""
	if m.loading && m.currentToolName != "" {
		toolBox = renderToolCallBox(m.currentToolName, m.currentToolArgs.String(), m.width)
	}

	if toolBox != "" {
		return fmt.Sprintf("%s\n%s\n%s", m.viewport.View(), toolBox, m.textarea.View())
	}
	return fmt.Sprintf("%s\n\n%s", m.viewport.View(), m.textarea.View())
}
