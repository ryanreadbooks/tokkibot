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
	"prompts/TOOLS.md",
}

// context management for the agent
type ContextManager struct {
	systemPrompts string

	historyInjectOnce sync.Once
	messageList       []llmmodel.MessageParam
	sessionMgr        *SessionManager
	memoryMgr         *MemoryManager
}

type ContextManagerConfig struct {
	workspace string
}

func NewContextManage(ctx context.Context, c ContextManagerConfig) (*ContextManager, error) {
	sessionMgr := NewSessionManager(ctx, SessionManagerConfig{
		workspace:    c.workspace,
		saveInterval: 10 * time.Second,
	})

	memoryMgr := NewMemoryManager(MemoryManagerConfig{
		workspace: c.workspace,
	})

	mgr := &ContextManager{
		sessionMgr: sessionMgr,
		memoryMgr:  memoryMgr,
	}

	if err := mgr.bootstrapSystemPrompts(); err != nil {
		return nil, fmt.Errorf("failed to bootstrap system prompts: %w", err)
	}

	return mgr, nil
}

func (c *ContextManager) fillPromptsPlaceHolders(s string) string {
	return strings.ReplaceAll(s, "{{workspace}}", config.GetConfigDir())
}

// System prompts bootstrap
func (c *ContextManager) bootstrapSystemPrompts() error {
	// System prompts structure:
	//
	// 	system built-in prompts
	//
	//  ---
	//
	//  memory prompts

	const separator = "\n\n---\n\n"

	prompts := strings.Builder{}
	prompts.Grow(1024 * len(systemPromptList))

	// system built-in prompts
	for _, promptPath := range systemPromptList {
		promptPath = filepath.Join(config.GetConfigDir(), promptPath)
		content, err := os.ReadFile(promptPath)
		if err != nil {
			return err
		}

		_, err = prompts.Write(content)
		if err != nil {
			return err
		}

		// add separator
		_, err = prompts.WriteString(separator)
		if err != nil {
			return err
		}
	}

	// memory prompts
	memoryPrompt, err := c.memoryMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load memory prompts: %w", err)
	}

	prompts.WriteString(separator)
	prompts.WriteString(memoryPrompt)

	// TODO skill prompts

	c.systemPrompts = prompts.String()
	c.systemPrompts = c.fillPromptsPlaceHolders(c.systemPrompts)
	// the first one is system prompt
	c.messageList = append(c.messageList,
		llmmodel.NewSystemMessageParam(c.systemPrompts),
	)

	return nil
}

func (c *ContextManager) InitHistoryMessages(channel channelmodel.Type, chatId string) {
	// Load history session messages into message list for the first time
	c.historyInjectOnce.Do(func() {
		history, err := c.sessionMgr.GetSessionHistory(channel, chatId)
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
}

func (c *ContextManager) AppendUserMessage(inMsg *channelmodel.IncomingMessage) []llmmodel.MessageParam {
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
	c.messageList = append(c.messageList, llmmodel.NewToolMessageParam(toolCall.Id, result))
	c.sessionMgr.GetSession(inMsg.Channel, inMsg.ChatId).addToolMessage(toolCall.Id, result)
}

// Add an assistant message (responded from the LLM) to the message list.
func (c *ContextManager) AppendAssistantMessage(
	inMsg *channelmodel.IncomingMessage,
	msg *llmmodel.CompletionMessage,
) {
	c.messageList = append(c.messageList, llmmodel.NewAssistantMessageParam(msg.Content, msg.GetToolCallParams()))
	c.sessionMgr.GetSession(inMsg.Channel, inMsg.ChatId).addAssistantMessage(msg.Content, msg.ToolCalls)
}

func (c *ContextManager) GetMessageList() []llmmodel.MessageParam {
	return c.messageList
}
