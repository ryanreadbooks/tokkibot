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
	"github.com/ryanreadbooks/tokkibot/agent/ref/media"
	"github.com/ryanreadbooks/tokkibot/component/skill"
	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/llm/schema"
	"github.com/ryanreadbooks/tokkibot/llm/schema/param"
	"github.com/ryanreadbooks/tokkibot/pkg/dataurl"
	"github.com/ryanreadbooks/tokkibot/pkg/xstring"
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
	messageList       []param.Message
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

	// memory prompts (optional, not fatal if missing)
	if memoryPrompt := c.memoryMgr.Load(); memoryPrompt != "" {
		prompts.WriteString("\n\n")
		prompts.WriteString(memoryPrompt)
	}

	c.systemPrompts = prompts.String()
	c.systemPrompts = c.renderPrompts(c.systemPrompts)
	// the first one is system prompt
	c.messageList = append(c.messageList,
		param.NewSystemMessage(c.systemPrompts),
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
	msgParam *param.Message,
	addToAOF func(*session.AOFLog, *param.Message) error,
	addToContext func(*session.ContextLog, *param.Message) error,
) error {
	contextLog, err := c.contextLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return err
	}
	if err = addToContext(contextLog, msgParam); err != nil {
		return err
	}

	aofLog, err := c.aofLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return err
	}
	if err = addToAOF(aofLog, msgParam); err != nil {
		return err
	}

	c.messageList = append(c.messageList, *msgParam)
	return nil
}

type contentUnionWithKey struct {
	*param.ContentUnion
	Key string
}

func parseInputAttachments(attachments []*UserInputAttachment) ([]*contentUnionWithKey, error) {
	params := make([]*contentUnionWithKey, 0, len(attachments))
	for _, attachment := range attachments {
		switch attachment.Type {
		case ImageAttachment:
			data := dataurl.Base64Encode(attachment.Data)
			params = append(params, &contentUnionWithKey{
				ContentUnion: &param.ContentUnion{
					ImageURL: &param.ImageURL{
						URL: data,
					},
				},
				Key: attachment.Key,
			})
		case FileAttachment:
			// we consider file attachment as text string content if it is a file
			params = append(params, &contentUnionWithKey{
				ContentUnion: &param.ContentUnion{
					Text: &param.Text{
						Value: xstring.FromBytes(attachment.Data),
					},
				},
				Key: attachment.Key,
			})
		default:
			return nil, fmt.Errorf("unsupported attachment type yet: %s", attachment.Type)
		}
	}

	return params, nil
}

func (c *ContextManager) AppendUserMessage(inMsg *UserInput) ([]param.Message, error) {
	// check any attachments
	unionParamsWithKey, err := parseInputAttachments(inMsg.Attachments)
	if err != nil {
		return nil, err
	}

	logItem := session.LogItem{
		Id:      session.NewLogItemId(),
		Role:    param.RoleUser,
		Created: time.Now().Unix(),
		Metadata: &session.LogItemMeta{
			ImageRef: map[int]string{},
		},
	}

	var msgParam param.Message
	if len(unionParamsWithKey) > 0 {
		for idx, un := range unionParamsWithKey {
			if un != nil && un.ImageURL != nil && un.ImageURL.URL != "" && un.Key != "" {
				if mediaRefName, err := media.SaveMedia(xstring.ToBytes(un.ImageURL.URL), un.Key); err == nil {
					// we use ref
					logItem.Metadata.ImageRef[idx] = mediaRefName
				}
			}
		}

		unionParams := make([]*param.ContentUnion, 0, len(unionParamsWithKey))
		for _, un := range unionParamsWithKey {
			unionParams = append(unionParams, un.ContentUnion)
		}

		if inMsg.Content != "" {
			unionParams = append(unionParams, &param.ContentUnion{
				Text: &param.Text{
					Value: inMsg.Content,
				},
			})
		}
		msgParam = param.NewUserMessage(unionParams)
	} else {
		msgParam = param.NewUserMessage(inMsg.Content)
	}

	logItem.Message = &msgParam

	err = c.appendMessage(
		inMsg.Channel, inMsg.ChatId, &msgParam,
		func(aofLog *session.AOFLog, msg *param.Message) error {
			return aofLog.AddLogItem(logItem)
		},
		func(contextLog *session.ContextLog, msg *param.Message) error {
			return contextLog.AddLogItem(logItem)
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
	msgParam := param.NewToolMessage(toolCall.Id, result)
	logItem := session.LogItem{
		Id:      session.NewLogItemId(),
		Role:    param.RoleTool,
		Created: time.Now().Unix(),
		Message: &msgParam,
	}

	return c.appendMessage(
		inMsg.Channel, inMsg.ChatId, &msgParam,
		func(log *session.AOFLog, msg *param.Message) error {
			return log.AddLogItem(logItem)
		},
		func(log *session.ContextLog, msg *param.Message) error {
			return log.AddLogItem(logItem)
		},
	)
}

// Add an assistant message (responded from the LLM) to the message list.
func (c *ContextManager) AppendAssistantMessage(
	inMsg *UserInput,
	msg *schema.CompletionMessage,
) error {
	var reasoningContent *param.String
	if msg.ReasoningContent != "" {
		reasoningContent = &param.String{Value: msg.ReasoningContent}
	}
	msgParam := param.NewAssistantMessage(
		msg.Content,
		msg.GetToolCalls(),
		reasoningContent,
	)
	logItem := session.LogItem{
		Id:      session.NewLogItemId(),
		Role:    param.RoleAssistant,
		Created: time.Now().Unix(),
		Message: &msgParam,
	}

	return c.appendMessage(
		inMsg.Channel, inMsg.ChatId, &msgParam,
		func(log *session.AOFLog, msg *param.Message) error {
			return log.AddLogItem(logItem)
		},
		func(log *session.ContextLog, msg *param.Message) error {
			return log.AddLogItem(logItem)
		},
	)
}

func (c *ContextManager) GetMessageContext(channel, chatId string) (
	[]param.Message, error,
) {
	log, err := c.contextLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return nil, err
	}

	// In-memory logs already contain actual content (refs are only used for disk storage)
	logs := log.GetLogs()

	// +1 for system prompt
	msgList := make([]param.Message, 0, len(logs)+1)

	// System prompt always goes first
	msgList = append(msgList, param.NewSystemMessage(c.systemPrompts))

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

// ClearSession clears all messages in a session (keeps system prompt)
func (s *ContextManager) ClearSession(channel, chatId string) error {
	contextLog, err := s.contextLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return err
	}

	// clear context log (in-memory)
	contextLog.ResetLogsFromMessage(nil)

	// flush to disk
	if err := contextLog.Flush(s.contextLogManager.Workspace); err != nil {
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
	llmFunc func(context.Context, []param.Message) (string, error),
) error {
	contextLog, err := c.contextLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return err
	}

	// In-memory logs already contain actual content
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
		if prevMsg.Role() == param.RoleAssistant &&
			prevMsg.Assistant != nil &&
			len(prevMsg.Assistant.ToolCalls) > 0 {
			// Need to include following tool responses
			toolCallIds := make(map[string]bool)
			for _, tc := range prevMsg.Assistant.ToolCalls {
				if tc.Function != nil {
					toolCallIds[tc.Function.Id] = true
				}
			}

			// Scan forward to collect all tool responses
			matched := 0
			for j := endIdx; j < len(logs) && matched < len(toolCallIds); j++ {
				msg := logs[j].Message
				if msg.Role() == param.RoleTool && msg.Tool != nil {
					if toolCallIds[msg.Tool.ToolCallId] {
						matched++
						endIdx = j + 1
					}
				} else if msg.Role() == param.RoleAssistant {
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
	if startIdx < endIdx && logs[startIdx].Message.Role() == param.RoleTool {
		// Scan backward to find the assistant message with tool_calls
		for i := startIdx - 1; i >= 0; i-- {
			msg := logs[i].Message
			if msg.Role() == param.RoleAssistant &&
				msg.Assistant != nil &&
				len(msg.Assistant.ToolCalls) > 0 {
				// Found the assistant message, include it
				startIdx = i
				break
			}
		}

		// If we still start with a tool message (couldn't find assistant msg),
		// skip orphaned tool messages
		for startIdx < endIdx && logs[startIdx].Message.Role() == param.RoleTool {
			startIdx++
		}
	}

	if endIdx <= startIdx {
		// Nothing to summarize after adjustments
		return nil
	}

	toSummarize := make([]param.Message, 0, endIdx-startIdx)
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
	newMsgList := make([]param.Message, 0, 1+recentKeepCount)

	// Add summary as user message
	summaryMsg := param.NewUserMessage(
		fmt.Sprintf("[Conversation History Summary]\n%s\n[End of Summary, Recent Messages Follow]", summary),
	)
	newMsgList = append(newMsgList, summaryMsg)

	// Add recent messages
	for i := endIdx; i < len(logs); i++ {
		newMsgList = append(newMsgList, *logs[i].Message)
	}

	// Update context log with new messages
	contextLog.ResetLogsFromMessage(newMsgList)

	// Flush to disk
	if err := contextLog.Flush(c.contextLogManager.Workspace); err != nil {
		return fmt.Errorf("failed to flush after summarization: %w", err)
	}

	return nil
}
