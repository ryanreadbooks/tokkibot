package agent

import (
	"fmt"

	"github.com/ryanreadbooks/tokkibot/agent/context/session"
	"github.com/ryanreadbooks/tokkibot/component/skill"
	"github.com/ryanreadbooks/tokkibot/component/tool"
	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/llm/estimator"
	"github.com/ryanreadbooks/tokkibot/llm/factory"
	schema "github.com/ryanreadbooks/tokkibot/llm/schema"
)

func (a *Agent) RetrieveMessageHistory(channel, chatId string) (
	[]session.LogItem, error,
) {
	history, err := a.contextManager.GetMessageHistory(
		channel, chatId,
	)
	if err != nil {
		return nil, err
	}

	return history, nil
}

func (a *Agent) AvailableSkills() []*skill.Skill {
	return a.skillLoader.Skills()
}

func (a *Agent) GetSystemPrompt() string {
	return a.contextManager.GetSystemPrompt()
}

func (a *Agent) InitSession(channel, chatId string) error {
	return a.contextManager.InitSession(channel, chatId)
}

func (a *Agent) GetCurrentContextTokens(channel, chatId string) int64 {
	est := estimator.RoughEstimator{}
	a.cachedReqsMu.RLock()
	if req, ok := a.cachedReqs[channel+":"+chatId]; ok {
		a.cachedReqsMu.RUnlock()
		tokens, err := est.Estimate(a.cfg.RootCtx, req)
		if err != nil {
			return 0
		}

		return int64(tokens)
	}
	a.cachedReqsMu.RUnlock()

	// If no cached request, build a minimal request without triggering compaction
	msgList, err := a.contextManager.GetMessageContext(channel, chatId)
	if err != nil {
		return 0
	}

	// Create a temporary request for estimation only (without calling buildLLMMessageRequest)
	fakeReq := schema.NewRequest(a.cfg.Model, msgList)
	fakeReq.Tools = a.buildLLMTools()

	tokens, err := est.Estimate(a.cfg.RootCtx, fakeReq)
	if err != nil {
		return 0
	}

	return int64(tokens)
}

func (a *Agent) ListMcpTools() []*tool.McpTool {
	if a.mcpLoaded.Load() {
		return a.mcpManager.ListTools()
	}
	return nil
}

func (a *Agent) ListMcpServers() []*tool.McpServerStatus {
	if a.mcpLoaded.Load() {
		return a.mcpManager.ListServers()
	}
	return nil
}

func (a *Agent) GetModel() string {
	return a.cfg.Model
}

func (a *Agent) GetProvider() string {
	return a.cfg.Provider
}

func (a *Agent) SetModel(model string) {
	a.cfg.Model = model
}

func (a *Agent) SetProvider(provider string) error {
	return a.SetProviderAndModel(provider, "")
}

// SetProviderAndModel switches the provider and model, recreating the LLM client
func (a *Agent) SetProviderAndModel(provider, model string) error {
	cfg := config.GetConfig()
	providerCfg, ok := cfg.Providers[provider]
	if !ok {
		return fmt.Errorf("provider not found: %s", provider)
	}

	if model == "" {
		model = providerCfg.DefaultModel
	}

	// Create new LLM client with the new provider config
	newLLM, err := factory.NewLLM(
		factory.WithAPIKey(providerCfg.ApiKey),
		factory.WithBaseURL(providerCfg.BaseURL),
		factory.WithStyle(factory.Style(providerCfg.Style)),
	)
	if err != nil {
		return fmt.Errorf("failed to create LLM client: %w", err)
	}

	// Update agent config and LLM client
	a.cfg.Provider = provider
	a.cfg.Model = model
	a.llm = newLLM

	return nil
}

func (a *Agent) GetToolCount() int {
	a.toolsMu.RLock()
	defer a.toolsMu.RUnlock()
	return len(a.tools)
}

func (a *Agent) GetMcpToolCount() int {
	if a.mcpLoaded.Load() {
		return len(a.mcpManager.ListTools())
	}
	return 0
}

// ClearContext clears all messages in a session
func (a *Agent) ClearContext(channel, chatId string) error {
	// Clear cached request for token estimation
	cacheKey := channel + ":" + chatId
	a.cachedReqsMu.Lock()
	delete(a.cachedReqs, cacheKey)
	a.cachedReqsMu.Unlock()

	return a.contextManager.ClearSession(channel, chatId)
}
