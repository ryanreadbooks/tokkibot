package agent

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ryanreadbooks/tokkibot/agent"
	"github.com/ryanreadbooks/tokkibot/channel"
	"github.com/ryanreadbooks/tokkibot/channel/cli"
	channelmodel "github.com/ryanreadbooks/tokkibot/channel/model"
	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/llm/factory"
)

func prepareAgent(ctx context.Context) (
	ag *agent.Agent,
	bus *channel.Bus,
	err error,
) {
	cfg, err := config.LoadConfig()
	if err != nil {
		err = fmt.Errorf("failed to load config: %w", err)
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
		RootCtx: ctx,
		Model:   model,
	})

	return ag, bus, nil
}

func restoreHistory(ag *agent.Agent) ([]string, error) {
	history := make([]string, 0, 128)
	// resume history if provided
	if resumeSessionChatId == "" {
		agentChatId = uuid.New().String()
	} else {
		agentChatId = resumeSessionChatId
		historyMessages, err := ag.RetrieveSession(channelmodel.ChannelCLI, resumeSessionChatId)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve session: %w", err)
		}
		for _, msg := range historyMessages {
			if msg.IsFromUser() {
				history = append(history, youStyle.Render(youPrefix)+msg.Content)
			} else if msg.IsFromAssistant() {
				history = append(history, agentStyle.Render(agentPrefix)+msg.Content)
			}
		}
	}

	return history, nil
}
