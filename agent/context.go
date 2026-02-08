package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	channelmodel "github.com/ryanreadbooks/tokkibot/channel/model"
	"github.com/ryanreadbooks/tokkibot/config"
	llmmodel "github.com/ryanreadbooks/tokkibot/llm/model"
)

var systemPromptList = []string{
	"prompts/AGENTS.md",
}

// context management for the agent
type ContextManager struct {
	systemPrompts string

	historyInjectOnce sync.Once
	messageList       []llmmodel.MessageParam
	sessionMgr        *SessionManager
}

type ContextManagerConfig struct {
	workspace string
}

func NewContextManage(ctx context.Context, c ContextManagerConfig) (*ContextManager, error) {
	sessionMgr := NewSessionManager(ctx, SessionManagerConfig{
		workspace:    c.workspace,
		saveInterval: 10 * time.Second,
	})
	mgr := &ContextManager{
		sessionMgr: sessionMgr,
	}

	if err := mgr.bootstrapSystemPrompts(); err != nil {
		return nil, fmt.Errorf("failed to bootstrap system prompts: %w", err)
	}

	return mgr, nil
}

func (c *ContextManager) bootstrapSystemPrompts() error {
	prompts := strings.Builder{}
	prompts.Grow(512 * len(systemPromptList))
	for idx, promptPath := range systemPromptList {
		promptPath = filepath.Join(config.GetConfigDir(), promptPath)

		content, err := os.ReadFile(promptPath)
		if err != nil {
			return err
		}

		_, err = prompts.Write(content)
		if err != nil {
			return err
		}

		if idx < len(systemPromptList)-1 {
			// add separator
			_, err = prompts.WriteString("\n\n---\n\n")
			if err != nil {
				return err
			}
		}
	}

	c.systemPrompts = prompts.String()
	// the first one is system prompt
	c.messageList = append(c.messageList,
		llmmodel.NewSystemMessageParam(c.systemPrompts),
	)

	return nil
}

func (c *ContextManager) NextMessage(inMsg *channelmodel.IncomingMessage) []llmmodel.MessageParam {
	// Load history session messages into message list for the first time
	c.historyInjectOnce.Do(func() {
		history, err := c.sessionMgr.GetSessionHistory(inMsg.Channel, inMsg.ChatId)
		if err == nil {
			// concat the last 50 messages
			l := min(len(history), 50)
			history = history[len(history)-l:]

			for _, msg := range history {
				switch msg.Role {
				case llmmodel.RoleUser:
					c.messageList = append(c.messageList, llmmodel.NewUserMessageParam(msg.Content))
				case llmmodel.RoleAssistant:
					val := msg.Extras["tool_calls"]
					var toolCalls []*llmmodel.ToolCallParam
					if tch, ok := val.([]llmmodel.CompletionToolCall); ok {
						toolCalls = make([]*llmmodel.ToolCallParam, 0, len(tch))
						for _, toolCall := range tch {
							toolCalls = append(toolCalls, toolCall.ToToolCallParam())
						}
					}
					c.messageList = append(c.messageList, llmmodel.NewAssistantMessageParam(msg.Content, toolCalls))
				case llmmodel.RoleTool:
					callId, _ := msg.Extras["tool_call_id"].(string)
					c.messageList = append(c.messageList, llmmodel.NewToolMessageParam(callId, msg.Content))
				}
			}
		}
	})

	// we also should store the incoming message for future conversation
	userMsg := llmmodel.NewUserMessageParam(inMsg.Content)
	c.messageList = append(c.messageList, userMsg)

	c.sessionMgr.GetSession(inMsg.Channel, inMsg.ChatId).addUserMessage(inMsg.Content)

	return c.messageList
}

// Add a tool call result (usually generated locally) to the message list.
func (c *ContextManager) AppendToolResult(
	inMsg *channelmodel.IncomingMessage,
	toolCall *llmmodel.CompletionToolCall,
	result string, // the result of the toolCall with id
) {
	c.messageList = append(c.messageList, llmmodel.NewToolMessageParam(
		toolCall.Id,
		result,
	))

	c.sessionMgr.GetSession(inMsg.Channel, inMsg.ChatId).addToolMessage(toolCall.Id, result)
}

// Add an assistant message (responded from the LLM) to the message list.
func (c *ContextManager) AppendAssistantMessage(
	inMsg *channelmodel.IncomingMessage,
	msg *llmmodel.CompletionMessage,
) {
	c.messageList = append(c.messageList, llmmodel.NewAssistantMessageParam(
		msg.Content,
		msg.GetToolCallParams(),
	))

	c.sessionMgr.GetSession(inMsg.Channel, inMsg.ChatId).
		addAssistantMessage(msg.Content, msg.ToolCalls)
}
