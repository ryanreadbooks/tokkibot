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

// appendMessage is a helper to add a message to both logs
func (c *ContextManager) appendMessage(
	channel, chatId string,
	msgParam *schema.MessageParam,
	addToAOF func(*session.AOFLog, *schema.MessageParam) error,
	addToContext func(*session.ContextLog, *schema.MessageParam) error,
) error {
	aofLog, err := c.aofLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return err
	}
	if err = addToAOF(aofLog, msgParam); err != nil {
		return err
	}

	contextLog, err := c.contextLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return err
	}
	if err = addToContext(contextLog, msgParam); err != nil {
		return err
	}

	c.messageList = append(c.messageList, *msgParam)
	return nil
}

func (c *ContextManager) AppendUserMessage(inMsg *UserInput) ([]schema.MessageParam, error) {
	userMsg := schema.NewUserMessageParam(inMsg.Content)
	err := c.appendMessage(
		inMsg.Channel, inMsg.ChatId, &userMsg,
		func(log *session.AOFLog, msg *schema.MessageParam) error {
			return log.AddUserMessage(msg)
		},
		func(log *session.ContextLog, msg *schema.MessageParam) error {
			return log.AddUserMessage(msg)
		},
	)
	if err != nil {
		return nil, err
	}
	return c.messageList, nil
}

// Add a tool call result (usually generated locally) to the message list.
func (c *ContextManager) AppendToolResult(
	inMsg *UserInput,
	toolCall *schema.CompletionToolCall,
	result string,
) error {
	msgParam := schema.NewToolMessageParam(toolCall.Id, result)
	return c.appendMessage(
		inMsg.Channel, inMsg.ChatId, &msgParam,
		func(log *session.AOFLog, msg *schema.MessageParam) error {
			return log.AddToolMessage(msg)
		},
		func(log *session.ContextLog, msg *schema.MessageParam) error {
			return log.AddToolMessage(msg)
		},
	)
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
	return c.appendMessage(
		inMsg.Channel, inMsg.ChatId, &msgParam,
		func(log *session.AOFLog, msg *schema.MessageParam) error {
			return log.AddAssistantMessage(msg)
		},
		func(log *session.ContextLog, msg *schema.MessageParam) error {
			return log.AddAssistantMessage(msg)
		},
	)
}

func (c *ContextManager) GetMessageContext(channel, chatId string) (
	[]schema.MessageParam, error,
) {
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

// CompressToolCalls compresses tool call result messages in the context to ref files
func (c *ContextManager) CompressToolCalls(channel, chatId string, count int) (int, error) {
	contextLog, err := c.contextLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return 0, err
	}
	
	compressed, err := contextLog.CompressToolCalls(count)
	if err != nil {
		return 0, err
	}
	
	// Flush to disk
	if err := contextLog.Flush(c.contextLogManager.Workspace); err != nil {
		return compressed, fmt.Errorf("failed to flush after compression: %w", err)
	}
	
	return compressed, nil
}

// SummarizeHistory uses LLM to summarize conversation history and replace old messages
func (c *ContextManager) SummarizeHistory(
	ctx context.Context,
	channel, chatId string,
	llmFunc func(context.Context, []schema.MessageParam) (string, error),
) error {
	contextLog, err := c.contextLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return err
	}

	logs := contextLog.GetLogs()
	if len(logs) < 10 {
		// Too few messages to summarize
		return nil
	}

	// Keep recent messages, summarize the older part
	// Note: contextLog doesn't include system prompt, it's managed separately
	recentKeepCount := 5 // keep last 5 messages

	if len(logs) <= recentKeepCount {
		return nil
	}

	// Extract messages to summarize (exclude recent messages)
	startIdx := 0
	endIdx := len(logs) - recentKeepCount
	
	// Ensure message sequence integrity: if endIdx-1 is assistant with tool_calls,
	// we must include all corresponding tool responses to avoid incomplete sequences
	for endIdx < len(logs) {
		if endIdx <= startIdx {
			break
		}
		
		prevMsg := logs[endIdx-1].Message
		if prevMsg.Role() == schema.RoleAssistant && 
		   prevMsg.AssistantMessageParam != nil && 
		   len(prevMsg.AssistantMessageParam.ToolCalls) > 0 {
			// Need to include following tool responses
			toolCallIds := make(map[string]bool)
			for _, tc := range prevMsg.AssistantMessageParam.ToolCalls {
				if tc.Function != nil {
					toolCallIds[tc.Function.Id] = true
				}
			}
			
			// Scan forward to collect all tool responses
			matched := 0
			for j := endIdx; j < len(logs) && matched < len(toolCallIds); j++ {
				msg := logs[j].Message
				if msg.Role() == schema.RoleTool && msg.ToolMessageParam != nil {
					if toolCallIds[msg.ToolMessageParam.ToolCallId] {
						matched++
						endIdx = j + 1
					}
				} else if msg.Role() == schema.RoleAssistant {
					// Stop if we hit another assistant message
					break
				}
			}
			break
		} else {
			break
		}
	}
	
	// Ensure startIdx doesn't begin with orphaned tool messages
	// If first message is a tool message, find its corresponding assistant message
	if startIdx < endIdx && logs[startIdx].Message.Role() == schema.RoleTool {
		// Scan backward to find the assistant message with tool_calls
		for i := startIdx - 1; i >= 0; i-- {
			msg := logs[i].Message
			if msg.Role() == schema.RoleAssistant &&
			   msg.AssistantMessageParam != nil &&
			   len(msg.AssistantMessageParam.ToolCalls) > 0 {
				// Found the assistant message, include it
				startIdx = i
				break
			}
		}
		
		// If we still start with a tool message (couldn't find assistant msg),
		// skip orphaned tool messages
		for startIdx < endIdx && logs[startIdx].Message.Role() == schema.RoleTool {
			startIdx++
		}
	}
	
	if endIdx <= startIdx {
		// Nothing to summarize after adjustments
		return nil
	}
	
	toSummarize := make([]schema.MessageParam, 0, endIdx-startIdx)
	for i := startIdx; i < endIdx; i++ {
		toSummarize = append(toSummarize, *logs[i].Message)
	}

	// Call LLM to summarize
	summary, err := llmFunc(ctx, toSummarize)
	if err != nil {
		return fmt.Errorf("failed to summarize history: %w", err)
	}

	// Create new message list: summary + recent messages
	// Note: system prompt is managed separately, not in contextLog
	newMsgList := make([]schema.MessageParam, 0, 1+recentKeepCount)
	
	// Add summary as user message
	summaryMsg := schema.NewUserMessageParam(
		fmt.Sprintf("[Conversation History Summary]\n%s\n[End of Summary, Recent Messages Follow]", summary),
	)
	newMsgList = append(newMsgList, summaryMsg)
	
	// Add recent messages
	for i := endIdx; i < len(logs); i++ {
		newMsgList = append(newMsgList, *logs[i].Message)
	}

	// Update context log with new messages
	contextLog.ResetLogsFromParam(newMsgList)
	
	// Flush to disk
	if err := contextLog.Flush(c.contextLogManager.Workspace); err != nil {
		return fmt.Errorf("failed to flush after summarization: %w", err)
	}

	return nil
}
