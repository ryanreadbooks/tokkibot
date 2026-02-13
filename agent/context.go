package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"text/template"
	"time"

	channelmodel "github.com/ryanreadbooks/tokkibot/channel/model"
	"github.com/ryanreadbooks/tokkibot/component/skill"
	"github.com/ryanreadbooks/tokkibot/config"
	llmmodel "github.com/ryanreadbooks/tokkibot/llm/model"

	"github.com/mitchellh/mapstructure"
)

type promptBuiltinInfo struct {
	Cwd             string // current working directory
	Workspace       string // system workspace
	Now             string
	Runtime         string
	AvailableSkills string
}

var systemPromptList = []string{
	"prompts/AGENTS.md",
	"prompts/IDENTITY.md",
	"prompts/TOOLS.md",
}

// context management for the agent
type ContextManager struct {
	systemPrompts string

	historyInjectOnce sync.Once
	messageList       []llmmodel.MessageParam
	sessionMgr        *SessionManager
	memoryMgr         *MemoryManager
	skillLoader       *skill.Loader
}

type ContextManagerConfig struct {
	workspace string
}

func NewContextManage(
	ctx context.Context,
	c ContextManagerConfig,
	sessionManager *SessionManager,
	memoryManager *MemoryManager,
	skillLoader *skill.Loader,
) (*ContextManager, error) {
	mgr := &ContextManager{
		sessionMgr:  sessionManager,
		memoryMgr:   memoryManager,
		skillLoader: skillLoader,
	}

	if err := mgr.bootstrapSystemPrompts(); err != nil {
		return nil, fmt.Errorf("failed to bootstrap system prompts: %w", err)
	}

	return mgr, nil
}

// Render placeholder variables in prompt.
//
// Available variables see [promptBuiltinInfo]
func (c *ContextManager) renderPrompts(s string) string {
	tmpl, err := template.New("prompts").Parse(s)
	if err != nil {
		panic(err)
	}

	builtinInfo := promptBuiltinInfo{
		Cwd:             config.GetProjectDir(),
		Workspace:       config.GetWorkspaceDir(),
		Now:             time.Now().String(),
		Runtime:         runtime.GOOS,
		AvailableSkills: skill.SkillsAsPrompt(c.skillLoader.Skills()),
	}

	var bd strings.Builder
	err = tmpl.Execute(&bd, builtinInfo)
	if err != nil {
		panic(err)
	}

	return bd.String()
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
		promptPath = filepath.Join(config.GetWorkspaceDir(), promptPath)
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
	prompts.WriteString(memoryPrompt)

	c.systemPrompts = prompts.String()
	c.systemPrompts = c.renderPrompts(c.systemPrompts)
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
					val := msg.Extras["tool_calls"] // []map[string]any
					var (
						toolCalls           []*llmmodel.ToolCallParam
						completionToolCalls []llmmodel.CompletionToolCall
					)

					err = mapstructure.Decode(val, &completionToolCalls)
					if err == nil {
						toolCalls = make([]*llmmodel.ToolCallParam, 0, len(completionToolCalls))
						for _, toolCall := range completionToolCalls {
							// we have to make sure tool call id exists
							if toolCall.Id != "" {
								toolCalls = append(toolCalls, toolCall.ToToolCallParam())
							}
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
