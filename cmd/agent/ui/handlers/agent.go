package handlers

import (
	"context"
	"time"

	"github.com/ryanreadbooks/tokkibot/agent"
	"github.com/ryanreadbooks/tokkibot/agent/context/session"
	cliadapter "github.com/ryanreadbooks/tokkibot/channel/adapter/cli"
	"github.com/ryanreadbooks/tokkibot/channel/model"
	"github.com/ryanreadbooks/tokkibot/cmd/agent/ui/types"
)

// AgentHandler handles agent interactions via CLI adapter
type AgentHandler struct {
	agent      *agent.Agent
	cliAdapter *cliadapter.CLIAdapter
	channel    string
	chatID     string
}

// NewAgentHandler creates a new agent handler with CLI adapter
func NewAgentHandler(ag *agent.Agent, adapter *cliadapter.CLIAdapter) *AgentHandler {
	return &AgentHandler{
		agent:      ag,
		cliAdapter: adapter,
		channel:    model.CLI.String(),
		chatID:     adapter.ChatID(),
	}
}

// StreamResult wraps the streaming channels
type StreamResult struct {
	Content  <-chan *StreamContent
	ToolCall <-chan *StreamToolCall
}

type StreamContent struct {
	Round            int
	Content          string
	ReasoningContent string
}

type StreamToolCall struct {
	Round     int
	Name      string
	Arguments string
}

// SendMessage sends a message via adapter to gateway and returns streaming result
func (h *AgentHandler) SendMessage(ctx context.Context, content string) *StreamResult {
	result := h.cliAdapter.SendUserMessage(ctx, content)

	contentCh := make(chan *StreamContent, 16)
	toolCallCh := make(chan *StreamToolCall, 16)

	go func() {
		for c := range result.Content {
			contentCh <- &StreamContent{
				Round:            c.Round,
				Content:          c.Content,
				ReasoningContent: c.ReasoningContent,
			}
		}
		close(contentCh)
	}()

	go func() {
		for tc := range result.ToolCall {
			toolCallCh <- &StreamToolCall{
				Round:     tc.Round,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			}
		}
		close(toolCallCh)
	}()

	return &StreamResult{
		Content:  contentCh,
		ToolCall: toolCallCh,
	}
}

// GetTokens returns current token count
func (h *AgentHandler) GetTokens() int64 {
	return h.agent.GetCurrentContextTokens(h.channel, h.chatID)
}

// LoadHistory loads conversation history
func (h *AgentHandler) LoadHistory() ([]types.Message, error) {
	history, err := h.agent.RetrieveMessageHistory(h.channel, h.chatID)
	if err != nil {
		return nil, err
	}

	messages := make([]types.Message, 0, len(history))
	for _, item := range history {
		converted := convertSessionLogItem(item)
		messages = append(messages, converted...)
	}

	return messages, nil
}

// InitSession initializes the agent session
func (h *AgentHandler) InitSession() error {
	return h.agent.InitSession(h.channel, h.chatID)
}

// GetAgent returns the underlying agent
func (h *AgentHandler) GetAgent() *agent.Agent {
	return h.agent
}

// convertSessionLogItem converts a session log item to UI messages
// Returns multiple messages when assistant has tool calls
func convertSessionLogItem(item session.LogItem) []types.Message {
	if item.Message == nil {
		return nil
	}

	timestamp := time.Unix(item.Created, 0)

	if item.IsFromUser() && item.Message.User != nil {
		return []types.Message{{
			Role:      types.RoleUser,
			Content:   item.Message.User.String.GetValue(),
			Timestamp: timestamp,
		}}
	}

	if item.IsFromAssistant() && item.Message.Assistant != nil {
		var messages []types.Message
		assistantParam := item.Message.Assistant

		// Extract content and reasoning
		content := ""
		reasoningContent := ""
		if assistantParam.Content != nil {
			content = assistantParam.Content.GetValue()
		}
		if assistantParam.ReasoningContent != nil {
			reasoningContent = assistantParam.ReasoningContent.GetValue()
		}

		hasToolCalls := len(assistantParam.ToolCalls) > 0

		// Order: thinking -> tool calls -> content
		// 1. If has tool calls, show thinking first (separate from content)
		if hasToolCalls && reasoningContent != "" {
			messages = append(messages, types.Message{
				Role:             types.RoleAssistant,
				ReasoningContent: reasoningContent,
				Timestamp:        timestamp,
			})
		}

		// 2. Add tool call messages
		if hasToolCalls {
			for _, tc := range assistantParam.ToolCalls {
				if tc.Function != nil {
					messages = append(messages, types.Message{
						Role:          types.RoleToolCall,
						ToolName:      tc.Function.Name,
						ToolArguments: tc.Function.Arguments,
						ToolComplete:  true,
						Timestamp:     timestamp,
					})
				}
			}
		}

		// 3. Add content (and reasoning if no tool calls)
		if hasToolCalls {
			// Only content after tool calls (thinking already shown)
			if content != "" {
				messages = append(messages, types.Message{
					Role:      types.RoleAssistant,
					Content:   content,
					Timestamp: timestamp,
				})
			}
		} else {
			// No tool calls - show content and reasoning together
			if content != "" || reasoningContent != "" {
				messages = append(messages, types.Message{
					Role:             types.RoleAssistant,
					Content:          content,
					ReasoningContent: reasoningContent,
					Timestamp:        timestamp,
				})
			}
		}

		return messages
	}

	return nil
}
