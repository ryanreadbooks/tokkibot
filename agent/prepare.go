package agent

import (
	"context"
	"fmt"

	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/llm/factory"
)

// PrepareOption allows customizing agent preparation
type PrepareOption func(*Config)

// WithWorkspace sets a custom workspace dir (e.g. __cron uses main's workspace)
func WithWorkspace(workspace string) PrepareOption {
	return func(c *Config) {
		c.WorkspaceDir = workspace
	}
}

func WithEnableCwdAccess(enable bool) PrepareOption {
	return func(c *Config) {
		c.EnableCwdAccess = enable
	}
}

// WithSessionDir sets a custom session dir (e.g. __cron uses global sessions dir)
func WithSessionDir(sessionDir string) PrepareOption {
	return func(c *Config) {
		c.SessionDir = sessionDir
	}
}

func Prepare(ctx context.Context, agentName string, opts ...PrepareOption) (ag *Agent, err error) {
	globalCfg := config.GetConfig()

	entry := config.GetAgentEntry(agentName)
	if entry == nil {
		// For virtual agents (e.g. __cron), fall back to main agent's provider settings
		entry = config.GetAgentEntry(config.MainAgentName)
		if entry == nil {
			return nil, fmt.Errorf("agent %s not found in config and no main agent fallback", agentName)
		}
	}

	providerName := entry.Provider
	provider, ok := globalCfg.Providers[providerName]
	if !ok {
		return nil, fmt.Errorf("provider %s not found for agent %s", providerName, agentName)
	}

	model := entry.Model
	if model == "" {
		model = provider.DefaultModel
	}

	llm, err := factory.NewLLM(
		factory.WithAPIKey(provider.ApiKey),
		factory.WithBaseURL(provider.BaseURL),
		factory.WithStyle(factory.Style(provider.Style)),
	)
	if err != nil {
		err = fmt.Errorf("failed to create llm: %w", err)
		return
	}

	agCfg := Config{
		RootCtx:      ctx,
		Name:         agentName,
		Provider:     providerName,
		Model:        model,
		MaxIteration: entry.MaxIteration,
		Sandbox:      entry.Sandbox,
	}
	for _, opt := range opts {
		opt(&agCfg)
	}

	ag = NewAgent(llm, agCfg)

	return ag, nil
}
