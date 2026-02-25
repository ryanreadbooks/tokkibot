package agent

import (
	"context"
	"fmt"

	"github.com/ryanreadbooks/tokkibot/agent"
	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/llm/factory"
)

func prepareAgent(ctx context.Context) (
	ag *agent.Agent,
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

	ag = agent.NewAgent(llm, agent.AgentConfig{
		RootCtx: ctx,
		Model:   model,
	})

	return ag, nil
}

