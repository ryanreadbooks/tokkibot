package context

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

	"github.com/ryanreadbooks/tokkibot/agent/context/session"
	"github.com/ryanreadbooks/tokkibot/component/skill"
	"github.com/ryanreadbooks/tokkibot/config"
	schema "github.com/ryanreadbooks/tokkibot/llm/schema"
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
	messageList       []schema.MessageParam
	aofLogManager     *session.AOFLogManager
	contextLogManager *session.ContextLogManager
	memoryMgr         *MemoryManager
	skillLoader       *skill.Loader
}

type ContextManagerConfig struct {
	Workspace string
}

func NewContextManager(
	ctx context.Context,
	c ContextManagerConfig,
	skillLoader *skill.Loader,
) (*ContextManager, error) {
	sessionCfg := session.LogManagerConfig{
		Workspace: filepath.Join(c.Workspace, "sessions"),
	}
	aofLogMgr := session.NewAOFLogManager(ctx, sessionCfg)
	contextLogMgr := session.NewContextLogManager(ctx, sessionCfg)
	memoryMgr := NewMemoryManager(MemoryManagerConfig{
		Workspace: c.Workspace,
	})
	mgr := &ContextManager{
		aofLogManager:     aofLogMgr,
		contextLogManager: contextLogMgr,
		memoryMgr:         memoryMgr,
		skillLoader:       skillLoader,
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

	now := time.Now()
	nowWithWeekday := fmt.Sprintf("%s, %s", now.String(), now.Weekday().String())

	builtinInfo := promptBuiltinInfo{
		Cwd:             config.GetProjectDir(),
		Workspace:       config.GetWorkspaceDir(),
		Now:             nowWithWeekday,
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
		schema.NewSystemMessageParam(c.systemPrompts),
	)

	return nil
}

func (c *ContextManager) InitFromSessionLogs(channel, chatId string) {
	// Load history session messages into message list for the first time
	c.historyInjectOnce.Do(func() {
		var (
			history []session.LogItem
			err     error
		)
		history, err = c.aofLogManager.GetLogItems(channel, chatId)
		if err == nil {
			for _, msg := range history {
				c.messageList = append(c.messageList, *msg.Message)
			}
		}
	})
}

func (c *ContextManager) AppendUserMessage(inMsg *UserInput) ([]schema.MessageParam, error) {
	// we also should store the incoming message for future conversation
	userMsg := schema.NewUserMessageParam(inMsg.Content)
	aofLog, err := c.aofLogManager.GetOrCreate(inMsg.Channel, inMsg.ChatId)
	if err != nil {
		return nil, err
	}
	err = aofLog.AddUserMessage(&userMsg)
	if err != nil {
		return nil, err
	}
	contextLog, err := c.contextLogManager.GetOrCreate(inMsg.Channel, inMsg.ChatId)
	if err != nil {
		return nil, err
	}
	err = contextLog.AddUserMessage(&userMsg)
	if err != nil {
		return nil, err
	}

	c.messageList = append(c.messageList, userMsg)
	return c.messageList, nil
}

// Add a tool call result (usually generated locally) to the message list.
func (c *ContextManager) AppendToolResult(
	inMsg *UserInput,
	toolCall *schema.CompletionToolCall,
	result string, // the result of the toolCall with id
) error {
	msgParam := schema.NewToolMessageParam(toolCall.Id, result)
	aofLog, err := c.aofLogManager.GetOrCreate(inMsg.Channel, inMsg.ChatId)
	if err != nil {
		return err
	}
	err = aofLog.AddToolMessage(&msgParam)
	if err != nil {
		return err
	}
	contextLog, err := c.contextLogManager.GetOrCreate(inMsg.Channel, inMsg.ChatId)
	if err != nil {
		return err
	}
	err = contextLog.AddToolMessage(&msgParam)
	if err != nil {
		return err
	}
	c.messageList = append(c.messageList, msgParam)

	return nil
}

// Add an assistant message (responded from the LLM) to the message list.
func (c *ContextManager) AppendAssistantMessage(
	inMsg *UserInput,
	msg *schema.CompletionMessage,
) error {
	var reasoningContent *schema.StringParam
	if msg.ReasoningContent != "" {
		reasoningContent = &schema.StringParam{Value: msg.ReasoningContent}
	}
	msgParam := schema.NewAssistantMessageParam(
		msg.Content,
		msg.GetToolCallParams(),
		reasoningContent,
	)
	aofLog, err := c.aofLogManager.GetOrCreate(inMsg.Channel, inMsg.ChatId)
	if err != nil {
		return err
	}
	err = aofLog.AddAssistantMessage(&msgParam)
	if err != nil {
		return err
	}
	contextLog, err := c.contextLogManager.GetOrCreate(inMsg.Channel, inMsg.ChatId)
	if err != nil {
		return err
	}
	err = contextLog.AddAssistantMessage(&msgParam)
	if err != nil {
		return err
	}
	c.messageList = append(c.messageList, msgParam)

	return nil
}

func (c *ContextManager) GetMessageContext(channel, chatId string) (
	[]schema.MessageParam, error,
) {
	// return c.messageList
	log, err := c.contextLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return nil, err
	}

	logs := log.GetLogs()
	msgList := make([]schema.MessageParam, 0, len(logs))
	for _, log := range logs {
		msgList = append(msgList, *log.Message)
	}

	return msgList, nil
}

func (c *ContextManager) GetSystemPrompt() string {
	return c.systemPrompts
}

func (c *ContextManager) GetMessageHistory(channel, chatId string) (
	[]session.LogItem, error,
) {
	return c.aofLogManager.GetLogItems(channel, chatId)
}

func (s *ContextManager) InitSession(channel, chatId string) error {
	_, err := s.aofLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return err
	}

	_, err = s.contextLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return err
	}

	return nil
}
