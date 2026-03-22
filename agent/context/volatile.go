package context

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/ryanreadbooks/tokkibot/agent/context/session"
	agentref "github.com/ryanreadbooks/tokkibot/agent/ref"
	"github.com/ryanreadbooks/tokkibot/component/skill"
	"github.com/ryanreadbooks/tokkibot/llm/schema"
	"github.com/ryanreadbooks/tokkibot/llm/schema/param"
)

type volatileSessionState struct {
	contextLogs []session.LogItem
	aofLogs     []session.LogItem
}

// VolatileContextManager stores all conversation data in-memory only.
// No session data is persisted to disk.
type VolatileContextManager struct {
	agentWorkspace        string
	systemPromptsTemplate string
	systemPromptsMu       sync.RWMutex

	sessionsMu sync.RWMutex
	sessions   map[string]*volatileSessionState

	memoryMgr    *MemoryManager
	skillLoader  *skill.Loader
	systemPrompt string // non-empty overrides workspace prompt files
}

func NewVolatileContextManager(
	ctx context.Context,
	c ContextManagerConfig,
	skillLoader *skill.Loader,
) (*VolatileContextManager, error) {
	_ = ctx

	mgr := &VolatileContextManager{
		agentWorkspace: c.AgentWorkspace,
		sessions:       make(map[string]*volatileSessionState),
		memoryMgr:      NewMemoryManager(MemoryManagerConfig{Workspace: c.AgentWorkspace}),
		skillLoader:    skillLoader,
		systemPrompt:   c.SystemPromptTemplate,
	}

	if err := mgr.bootstrapSystemPrompts(); err != nil {
		return nil, fmt.Errorf("failed to bootstrap system prompts: %w", err)
	}

	return mgr, nil
}

func (c *VolatileContextManager) renderPrompts(s string) string {
	return renderPromptTemplate(c.agentWorkspace, c.skillLoader, s)
}

func (c *VolatileContextManager) bootstrapSystemPrompts() error {
	prompts, err := loadSystemPromptTemplate(c.agentWorkspace, c.systemPrompt, c.memoryMgr)
	if err != nil {
		return err
	}
	c.systemPromptsTemplate = prompts
	return nil
}

func (c *VolatileContextManager) getRenderedSystemPrompts() string {
	return c.renderPrompts(c.systemPromptsTemplate)
}

func (c *VolatileContextManager) InitFromSessionLogs(channel, chatId string) {
	c.sessionsMu.Lock()
	defer c.sessionsMu.Unlock()
	c.getOrCreateSessionLocked(channel, chatId)
}

func (c *VolatileContextManager) InitSession(channel, chatId string) error {
	c.sessionsMu.Lock()
	defer c.sessionsMu.Unlock()
	c.getOrCreateSessionLocked(channel, chatId)
	return nil
}

func (c *VolatileContextManager) appendMessage(
	channel, chatId string,
	logItem session.LogItem,
	writeToAOF bool,
) error {
	c.sessionsMu.Lock()
	defer c.sessionsMu.Unlock()

	st := c.getOrCreateSessionLocked(channel, chatId)
	st.contextLogs = append(st.contextLogs, logItem)
	if writeToAOF {
		st.aofLogs = append(st.aofLogs, logItem)
	}
	return nil
}

func (c *VolatileContextManager) AppendContextUserMessage(inMsg *UserInput) ([]param.Message, error) {
	logItem, err := buildUserLogItemFromInput(inMsg)
	if err != nil {
		return nil, err
	}

	if err = c.appendMessage(inMsg.Channel, inMsg.ChatId, logItem, false); err != nil {
		return nil, err
	}
	return c.getSessionMessages(inMsg.Channel, inMsg.ChatId), nil
}

func (c *VolatileContextManager) AppendUserMessage(inMsg *UserInput) ([]param.Message, error) {
	logItem, err := buildUserLogItemFromInput(inMsg)
	if err != nil {
		return nil, err
	}

	if err = c.appendMessage(inMsg.Channel, inMsg.ChatId, logItem, true); err != nil {
		return nil, err
	}
	return c.getSessionMessages(inMsg.Channel, inMsg.ChatId), nil
}

func (c *VolatileContextManager) AppendToolResult(
	inMsg *UserInput,
	toolCall *schema.CompletionToolCall,
	result string,
) error {
	logItem := buildToolResultLogItem(toolCall, result)
	return c.appendMessage(inMsg.Channel, inMsg.ChatId, logItem, true)
}

func (c *VolatileContextManager) AppendAssistantMessage(
	inMsg *UserInput,
	msg *schema.CompletionMessage,
) error {
	logItem := buildAssistantLogItem(msg)
	return c.appendMessage(inMsg.Channel, inMsg.ChatId, logItem, true)
}

func (c *VolatileContextManager) GetMessageContext(channel, chatId string) ([]param.Message, error) {
	logs := c.getContextLogs(channel, chatId)
	return buildMessageContextWithSystemPrompt(c.getRenderedSystemPrompts(), logs), nil
}

func (c *VolatileContextManager) GetSystemPrompt() string {
	return c.getRenderedSystemPrompts()
}

func (c *VolatileContextManager) GetMessageHistory(channel, chatId string) ([]session.LogItem, error) {
	c.sessionsMu.RLock()
	defer c.sessionsMu.RUnlock()

	st := c.sessions[sessionKey(channel, chatId)]
	if st == nil || len(st.aofLogs) == 0 {
		return nil, nil
	}

	out := make([]session.LogItem, len(st.aofLogs))
	copy(out, st.aofLogs)
	return out, nil
}

func (c *VolatileContextManager) ClearSession(channel, chatId string) error {
	c.sessionsMu.Lock()
	defer c.sessionsMu.Unlock()

	st := c.getOrCreateSessionLocked(channel, chatId)
	st.contextLogs = nil
	return nil
}

func (c *VolatileContextManager) CompressToolCalls(channel, chatId string, count int) (int, error) {
	c.sessionsMu.Lock()
	defer c.sessionsMu.Unlock()

	st := c.getOrCreateSessionLocked(channel, chatId)
	compressed := 0

	for i := range st.contextLogs {
		if compressed >= count {
			break
		}

		item := &st.contextLogs[i]
		if item.Role != param.RoleTool || item.Message == nil || item.Message.Tool == nil {
			continue
		}

		toolMsg := item.Message.Tool
		content := toolMsg.GetContent()
		if isAlreadyRefInVolatile(content) || len(content) <= 500 {
			continue
		}

		refName, err := agentref.Save(content)
		if err != nil {
			continue
		}

		toolMsg.String = &param.String{
			Value: refName + " (use load_ref tool to read full content)",
		}
		toolMsg.Texts = nil
		compressed++
	}

	return compressed, nil
}

func (c *VolatileContextManager) SummarizeHistory(
	ctx context.Context,
	channel, chatId string,
	llmFunc func(context.Context, []param.Message) (string, error),
) error {
	logs := c.getContextLogs(channel, chatId)
	newMsgList, changed, err := summarizeHistoryMessages(ctx, logs, llmFunc)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}

	c.sessionsMu.Lock()
	defer c.sessionsMu.Unlock()

	st := c.getOrCreateSessionLocked(channel, chatId)
	st.contextLogs = messagesToSessionLogItems(newMsgList)
	return nil
}

func (c *VolatileContextManager) getOrCreateSessionLocked(channel, chatId string) *volatileSessionState {
	key := sessionKey(channel, chatId)
	if st, ok := c.sessions[key]; ok {
		return st
	}

	st := &volatileSessionState{
		contextLogs: make([]session.LogItem, 0, 64),
		aofLogs:     make([]session.LogItem, 0, 64),
	}
	c.sessions[key] = st
	return st
}

func (c *VolatileContextManager) getContextLogs(channel, chatId string) []session.LogItem {
	c.sessionsMu.RLock()
	defer c.sessionsMu.RUnlock()

	st := c.sessions[sessionKey(channel, chatId)]
	if st == nil || len(st.contextLogs) == 0 {
		return nil
	}

	out := make([]session.LogItem, len(st.contextLogs))
	copy(out, st.contextLogs)
	return out
}

func (c *VolatileContextManager) getSessionMessages(channel, chatId string) []param.Message {
	logs := c.getContextLogs(channel, chatId)
	if len(logs) == 0 {
		return nil
	}

	msgs := make([]param.Message, 0, len(logs))
	for _, item := range logs {
		msgs = append(msgs, *item.Message)
	}
	return msgs
}

func sessionKey(channel, chatId string) string {
	return channel + "_" + chatId
}

func isAlreadyRefInVolatile(content string) bool {
	if !strings.HasPrefix(content, agentref.RefPrefix) {
		return false
	}
	return strings.Contains(content, "(use load_ref tool to read full content)") || len(content) < 100
}
