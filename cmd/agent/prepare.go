package agent

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/ryanreadbooks/tokkibot/agent"
	chmodel "github.com/ryanreadbooks/tokkibot/channel/model"
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

func restoreHistory(ag *agent.Agent) ([]uiMsg, error) {
	history := make([]uiMsg, 0, 128)
	// resume history if provided
	if resumeSessionChatId == "" {
		agentChatId = uuid.New().String()
	} else {
		agentChatId = resumeSessionChatId
	}

	// init
	err := ag.InitSession(chmodel.ChannelCLI.String(), agentChatId)
	if err != nil {
		return nil, err
	}

	historyMessages, err := ag.RetrieveMessageHistory(chmodel.ChannelCLI.String(), agentChatId)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to retrieve session: %w", err)
	}
	for _, msg := range historyMessages {
		if msg.IsFromUser() {
			history = append(history, uiMsg{
				role: roleUser,
				content: uiMsgContent{
					content: msg.Message.UserMessageParam.String.GetValue(),
				},
			})
		} else if msg.IsFromAssistant() {
			history = append(history, uiMsg{
				role: roleAssistant,
				content: uiMsgContent{
					content:          msg.Message.AssistantMessageParam.Content.GetValue(),
					reasoningContent: msg.Message.AssistantMessageParam.ReasoningContent.GetValue(),
				},
			})
		}
	}

	return history, nil
}
