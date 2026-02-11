package agent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ryanreadbooks/tokkibot/agent"
	"github.com/ryanreadbooks/tokkibot/channel"
	"github.com/ryanreadbooks/tokkibot/channel/cli"
	channelmodel "github.com/ryanreadbooks/tokkibot/channel/model"
	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/llm/factory"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	agentChatId         string
	resumeSessionChatId string

	oneTimeQuestion string
)

var AgentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Interact with tokkibot agent in a CLI.",
	Long:  "Interact with tokkibot agent in a CLI.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if oneTimeQuestion != "" {
			return runAgentOneTime(ctx, oneTimeQuestion)
		}

		return runAgent(ctx, args)
	},
}

func init() {
	AgentCmd.Flags().StringVar(&resumeSessionChatId, "resume", "", "To resume a existing session, provide the session id.")
	AgentCmd.Flags().StringVar(&oneTimeQuestion, "message", "", "To ask a one-time question, provide the message.")
}

func prepareAgent(ctx context.Context) (
	ag *agent.Agent,
	bus *channel.Bus,
	history []string,
	err error,
) {
	cfg, err := config.LoadConfig()
	if err != nil {
		err = fmt.Errorf("failed to load config: %w", err)
		return
	}

	cwd, err := os.Getwd()
	if err != nil {
		err = fmt.Errorf("failed to get current working directory: %w", err)
		return
	}

	model := cfg.Providers[cfg.DefaultProvider].DefaultModel
	apiKey := cfg.Providers[cfg.DefaultProvider].ApiKey
	baseURL := cfg.Providers[cfg.DefaultProvider].BaseURL

	// prepare llm
	llm, err := factory.NewLLM(
		factory.WithAPIKey(apiKey),
		factory.WithBaseURL(baseURL),
	)
	if err != nil {
		err = fmt.Errorf("failed to create llm: %w", err)
		return
	}

	bus = channel.NewBus()
	bus.RegisterIncomingChannel(cli.NewCLIInputChannel())
	bus.RegisterOutgoingChannel(cli.NewCLIOutputChannel())

	ag = agent.NewAgent(llm, bus, agent.AgentConfig{
		RootCtx:   ctx,
		WorkingDir: cwd,
		Model:     model,
	})

	// resume history if provided
	if resumeSessionChatId == "" {
		agentChatId = uuid.New().String()
	} else {
		agentChatId = resumeSessionChatId
		historyMessages, err := ag.RetrieveSession(channelmodel.ChannelCLI, resumeSessionChatId)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to retrieve session: %w", err)
		}
		for _, msg := range historyMessages {
			if msg.IsFromUser() {
				history = append(history, youStyle.Render(youPrefix)+msg.Content)
			} else if msg.IsFromAssistant() {
				history = append(history, agentStyle.Render(agentPrefix)+msg.Content)
			}
		}
	}

	return ag, bus, history, nil
}

func runAgentOneTime(ctx context.Context, message string) error {
	ag, bus, _, err := prepareAgent(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare agent: %w", err)
	}

	ag.Run(ctx) // run in background

	answerChan := make(chan string)

	go func() {
		select {
		case <-ctx.Done():
			return
		case msg := <-bus.GetOutgoingChannel(channelmodel.ChannelCLI).Wait(ctx):
			answerChan <- msg.Content
		}
	}()

	err = bus.GetIncomingChannel(channelmodel.ChannelCLI).Send(ctx, channelmodel.IncomingMessage{
		Channel: channelmodel.ChannelCLI,
		ChatId:  "one-time",
		Created: time.Now().Unix(),
		Content: message,
	})
	if err != nil {
		return fmt.Errorf("failed to send message to agent: %w", err)
	}

	answer := <-answerChan
	fmt.Println("Agent: ", answer)

	return nil
}

func runAgent(ctx context.Context, args []string) error {
	ag, bus, history, err := prepareAgent(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare agent: %w", err)
	}
	ag.Run(ctx) // run in background

	p := tea.NewProgram(initAgentModel(ctx, ag, bus, history))
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run agent: %w", err)
	}

	return nil
}

const (
	inputHeight = 3 // height of textarea area including borders
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
	errMsg           error
	agentResponseMsg string

	agentModel struct {
		ctx context.Context
		ag  *agent.Agent
		bus *channel.Bus

		messages []string
		loading  bool

		viewport viewport.Model
		textarea textarea.Model
		spinner  spinner.Model
		err      error
	}
)

func initAgentModel(ctx context.Context, ag *agent.Agent, bus *channel.Bus, history []string) agentModel {
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
		ctx:      ctx,
		ag:       ag,
		bus:      bus,
		messages: history,
		textarea: ta,
		viewport: vp,
		spinner:  sp,
	}
}

func (m agentModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.waitForAgentResponse())
}

func (m agentModel) waitForAgentResponse() tea.Cmd {
	return func() tea.Msg {
		ch := m.bus.GetOutgoingChannel(channelmodel.ChannelCLI).Wait(m.ctx)
		select {
		case <-m.ctx.Done():
			return nil
		case msg := <-ch:
			return agentResponseMsg(msg.Content)
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
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - inputHeight
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

	case agentResponseMsg:
		m.loading = false
		m.messages = append(m.messages, agentStyle.Render(agentPrefix)+string(msg))
		m.updateViewportContent()
		return m, tea.Batch(taCmd, vpCmd, m.waitForAgentResponse())

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

// updateViewportContent refreshes viewport with current messages
func (m *agentModel) updateViewportContent() {
	var content string
	if len(m.messages) > 0 {
		content = strings.Join(m.messages, "\n")
	}
	// append loading spinner if waiting for agent response
	if m.loading {
		loadingLine := agentStyle.Render(agentPrefix) + m.spinner.View()
		if content != "" {
			content += "\n" + loadingLine
		} else {
			content = loadingLine
		}
	}
	m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width).Render(content))
	m.viewport.GotoBottom()
}

func (m agentModel) View() string {
	return fmt.Sprintf("%s\n\n%s", m.viewport.View(), m.textarea.View())
}
