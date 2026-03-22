package context

import (
	"context"
	"fmt"
	"sync"

	"github.com/ryanreadbooks/tokkibot/agent/context/session"
	"github.com/ryanreadbooks/tokkibot/component/skill"
	"github.com/ryanreadbooks/tokkibot/llm/schema"
	"github.com/ryanreadbooks/tokkibot/llm/schema/param"
)

type PersistentContextManager struct {
	agentWorkspace        string
	systemPromptsTemplate string
	systemPromptsMu       sync.RWMutex

	historyInjectOnce  sync.Once
	contextMessageList []param.Message
	aofLogManager      *session.LogManager[*session.AOFLog]
	contextLogManager  *session.LogManager[*session.ContextLog]
	memoryMgr          *MemoryManager
	skillLoader        *skill.Loader
	systemPrompt       string // non-empty overrides workspace prompt files
}

type ContextManagerConfig struct {
	AgentName            string
	AgentWorkspace       string
	SessionDir           string
	SystemPromptTemplate string
	Volatile             bool
}

func NewPersistentContextManager(
	ctx context.Context,
	c ContextManagerConfig,
	skillLoader *skill.Loader,
) (*PersistentContextManager, error) {
	mgr := &PersistentContextManager{
		agentWorkspace:    c.AgentWorkspace,
		aofLogManager:     session.NewAOFLogManager(c.SessionDir),
		contextLogManager: session.NewContextLogManager(c.SessionDir),
		memoryMgr:         NewMemoryManager(MemoryManagerConfig{Workspace: c.AgentWorkspace}),
		skillLoader:       skillLoader,
		systemPrompt:      c.SystemPromptTemplate,
	}

	if err := mgr.bootstrapSystemPrompts(); err != nil {
		return nil, fmt.Errorf("failed to bootstrap system prompts: %w", err)
	}

	return mgr, nil
}

// renderPrompts renders template variables in prompt string.
// Available variables: see [promptBuiltinInfo].
func (c *PersistentContextManager) renderPrompts(s string) string {
	return renderPromptTemplate(c.agentWorkspace, c.skillLoader, s)
}

// bootstrapSystemPrompts loads system prompts template from workspace files and memory.
// Structure: [built-in prompts or override] + [memory prompts]
func (c *PersistentContextManager) bootstrapSystemPrompts() error {
	prompts, err := loadSystemPromptTemplate(c.agentWorkspace, c.systemPrompt, c.memoryMgr)
	if err != nil {
		return err
	}
	c.systemPromptsTemplate = prompts
	return nil
}

func (c *PersistentContextManager) getRenderedSystemPrompts() string {
	return c.renderPrompts(c.systemPromptsTemplate)
}

// --- Session init & history ---

func (c *PersistentContextManager) InitFromSessionLogs(channel, chatId string) {
	c.historyInjectOnce.Do(func() {
		history, err := c.getAOFLogItems(channel, chatId)
		if err == nil {
			for _, item := range history {
				c.contextMessageList = append(c.contextMessageList, *item.Message)
			}
		}
	})
}

func (c *PersistentContextManager) getAOFLogItems(channel, chatId string) ([]session.LogItem, error) {
	log, err := c.aofLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return nil, err
	}
	return log.RetrieveLogItems(c.aofLogManager.Workspace)
}

func (c *PersistentContextManager) InitSession(channel, chatId string) error {
	if _, err := c.aofLogManager.GetOrCreate(channel, chatId); err != nil {
		return err
	}
	_, err := c.contextLogManager.GetOrCreate(channel, chatId)
	return err
}

// --- Message append ---

// appendMessage writes a logItem to context log (always) and AOF log (when writeToAOF is true).
func (c *PersistentContextManager) appendMessage(
	channel, chatId string,
	logItem session.LogItem,
	writeToAOF bool,
) error {
	contextLog, err := c.contextLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return err
	}
	if err = contextLog.AddLogItem(logItem); err != nil {
		return err
	}

	if writeToAOF {
		aofLog, err := c.aofLogManager.GetOrCreate(channel, chatId)
		if err != nil {
			return err
		}
		if err = aofLog.AddLogItem(logItem); err != nil {
			return err
		}
	}

	c.contextMessageList = append(c.contextMessageList, *logItem.Message)
	return nil
}

func (c *PersistentContextManager) AppendContextUserMessage(inMsg *UserInput) ([]param.Message, error) {
	logItem, err := buildUserLogItemFromInput(inMsg)
	if err != nil {
		return nil, err
	}

	if err = c.appendMessage(inMsg.Channel, inMsg.ChatId, logItem, false); err != nil {
		return nil, err
	}
	return c.contextMessageList, nil
}

func (c *PersistentContextManager) AppendUserMessage(inMsg *UserInput) ([]param.Message, error) {
	logItem, err := buildUserLogItemFromInput(inMsg)
	if err != nil {
		return nil, err
	}

	if err = c.appendMessage(inMsg.Channel, inMsg.ChatId, logItem, true); err != nil {
		return nil, err
	}
	return c.contextMessageList, nil
}

func (c *PersistentContextManager) AppendToolResult(
	inMsg *UserInput,
	toolCall *schema.CompletionToolCall,
	result string,
) error {
	logItem := buildToolResultLogItem(toolCall, result)
	return c.appendMessage(inMsg.Channel, inMsg.ChatId, logItem, true)
}

func (c *PersistentContextManager) AppendAssistantMessage(
	inMsg *UserInput,
	msg *schema.CompletionMessage,
) error {
	logItem := buildAssistantLogItem(msg)
	return c.appendMessage(inMsg.Channel, inMsg.ChatId, logItem, true)
}

// --- Context query ---

func (c *PersistentContextManager) GetMessageContext(channel, chatId string) ([]param.Message, error) {
	log, err := c.contextLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return nil, err
	}

	logs := log.GetLogs()
	return buildMessageContextWithSystemPrompt(c.getRenderedSystemPrompts(), logs), nil
}

func (c *PersistentContextManager) GetSystemPrompt() string {
	return c.getRenderedSystemPrompts()
}

func (c *PersistentContextManager) GetMessageHistory(channel, chatId string) ([]session.LogItem, error) {
	return c.getAOFLogItems(channel, chatId)
}

// --- Context compaction ---

func (c *PersistentContextManager) ClearSession(channel, chatId string) error {
	contextLog, err := c.contextLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return err
	}

	contextLog.ResetLogsFromMessage(nil)
	return contextLog.Flush(c.contextLogManager.Workspace)
}

func (c *PersistentContextManager) CompressToolCalls(channel, chatId string, count int) (int, error) {
	contextLog, err := c.contextLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return 0, err
	}

	compressed, err := contextLog.CompressToolCalls(count)
	if err != nil {
		return 0, err
	}

	if err := contextLog.Flush(c.contextLogManager.Workspace); err != nil {
		return compressed, fmt.Errorf("failed to flush after compression: %w", err)
	}
	return compressed, nil
}

// SummarizeHistory uses LLM to summarize older messages and keep recent ones.
func (c *PersistentContextManager) SummarizeHistory(
	ctx context.Context,
	channel, chatId string,
	llmFunc func(context.Context, []param.Message) (string, error),
) error {
	contextLog, err := c.contextLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return err
	}

	logs := contextLog.GetLogs()
	newMsgList, changed, err := summarizeHistoryMessages(ctx, logs, llmFunc)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}

	contextLog.ResetLogsFromMessage(newMsgList)
	if err := contextLog.Flush(c.contextLogManager.Workspace); err != nil {
		return fmt.Errorf("failed to flush after summarization: %w", err)
	}
	return nil
}

// adjustSummarizeBounds ensures [startIdx, endIdx) doesn't split assistant+tool_call sequences.
func adjustSummarizeBounds(logs []session.LogItem, startIdx, endIdx int) (int, int) {
	// If the boundary cuts after an assistant message with tool_calls,
	// extend endIdx to include the corresponding tool responses.
	if endIdx > startIdx && endIdx < len(logs) {
		prevMsg := logs[endIdx-1].Message
		if prevMsg.Role() == param.RoleAssistant &&
			prevMsg.Assistant != nil &&
			len(prevMsg.Assistant.ToolCalls) > 0 {

			toolCallIds := make(map[string]bool)
			for _, tc := range prevMsg.Assistant.ToolCalls {
				if tc.Function != nil {
					toolCallIds[tc.Function.Id] = true
				}
			}

			matched := 0
			for j := endIdx; j < len(logs) && matched < len(toolCallIds); j++ {
				msg := logs[j].Message
				if msg.Role() == param.RoleTool && msg.Tool != nil && toolCallIds[msg.Tool.ToolCallId] {
					matched++
					endIdx = j + 1
				} else if msg.Role() == param.RoleAssistant {
					break
				}
			}
		}
	}

	// If startIdx begins with orphaned tool messages, find the owning assistant message
	// or skip past them.
	if startIdx < endIdx && logs[startIdx].Message.Role() == param.RoleTool {
		for i := startIdx - 1; i >= 0; i-- {
			msg := logs[i].Message
			if msg.Role() == param.RoleAssistant && msg.Assistant != nil && len(msg.Assistant.ToolCalls) > 0 {
				startIdx = i
				break
			}
		}
		for startIdx < endIdx && logs[startIdx].Message.Role() == param.RoleTool {
			startIdx++
		}
	}

	return startIdx, endIdx
}
