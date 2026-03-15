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
	Cwd             string
	Workspace       string
	Now             string
	Runtime         string
	AvailableSkills string
	DateWithTz      string
}

var systemPromptList = []string{
	"AGENTS.md",
	"IDENTITY.md",
	"TOOLS.md",
}

type ContextManager struct {
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
}

func NewContextManager(
	ctx context.Context,
	c ContextManagerConfig,
	skillLoader *skill.Loader,
) (*ContextManager, error) {
	mgr := &ContextManager{
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
func (c *ContextManager) renderPrompts(s string) string {
	tmpl, err := template.New("prompts").Parse(s)
	if err != nil {
		panic(err)
	}

	now := time.Now()
	var bd strings.Builder
	err = tmpl.Execute(&bd, promptBuiltinInfo{
		Cwd:             config.GetProjectDir(),
		Workspace:       c.agentWorkspace,
		Now:             fmt.Sprintf("%s, %s", now.String(), now.Weekday().String()),
		Runtime:         runtime.GOOS,
		AvailableSkills: skill.SkillsAsPrompt(c.skillLoader.Skills()),
		DateWithTz:      now.Format("2006-01-02 MST -0700"),
	})
	if err != nil {
		panic(err)
	}

	return bd.String()
}

// bootstrapSystemPrompts loads system prompts template from workspace files and memory.
// Structure: [built-in prompts or override] + [memory prompts]
func (c *ContextManager) bootstrapSystemPrompts() error {
	const separator = "\n\n---\n\n"

	var prompts strings.Builder
	prompts.Grow(4096)

	if c.systemPrompt != "" {
		prompts.WriteString(c.systemPrompt)
	} else {
		for _, name := range systemPromptList {
			content, err := os.ReadFile(filepath.Join(c.agentWorkspace, name))
			if err != nil {
				return err
			}
			prompts.Write(content)
			prompts.WriteString(separator)
		}
	}

	if memoryPrompt := c.memoryMgr.Load(); memoryPrompt != "" {
		prompts.WriteString("\n\n")
		prompts.WriteString(memoryPrompt)
	}

	c.systemPromptsTemplate = prompts.String()
	return nil
}

func (c *ContextManager) getRenderedSystemPrompts() string {
	return c.renderPrompts(c.systemPromptsTemplate)
}

// --- Session init & history ---

func (c *ContextManager) InitFromSessionLogs(channel, chatId string) {
	c.historyInjectOnce.Do(func() {
		history, err := c.getAOFLogItems(channel, chatId)
		if err == nil {
			for _, item := range history {
				c.contextMessageList = append(c.contextMessageList, *item.Message)
			}
		}
	})
}

func (c *ContextManager) getAOFLogItems(channel, chatId string) ([]session.LogItem, error) {
	log, err := c.aofLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return nil, err
	}
	return log.RetrieveLogItems(c.aofLogManager.Workspace)
}

func (c *ContextManager) InitSession(channel, chatId string) error {
	if _, err := c.aofLogManager.GetOrCreate(channel, chatId); err != nil {
		return err
	}
	_, err := c.contextLogManager.GetOrCreate(channel, chatId)
	return err
}

// --- Message append ---

// appendMessage writes a logItem to context log (always) and AOF log (when writeToAOF is true).
func (c *ContextManager) appendMessage(
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

func (c *ContextManager) AppendContextUserMessage(inMsg *UserInput) ([]param.Message, error) {
	logItem, err := c.buildUserLogItem(inMsg)
	if err != nil {
		return nil, err
	}

	if err = c.appendMessage(inMsg.Channel, inMsg.ChatId, logItem, false); err != nil {
		return nil, err
	}
	return c.contextMessageList, nil
}

func (c *ContextManager) AppendUserMessage(inMsg *UserInput) ([]param.Message, error) {
	logItem, err := c.buildUserLogItem(inMsg)
	if err != nil {
		return nil, err
	}

	if err = c.appendMessage(inMsg.Channel, inMsg.ChatId, logItem, true); err != nil {
		return nil, err
	}
	return c.contextMessageList, nil
}

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
	return c.appendMessage(inMsg.Channel, inMsg.ChatId, logItem, true)
}

func (c *ContextManager) AppendAssistantMessage(
	inMsg *UserInput,
	msg *schema.CompletionMessage,
) error {
	var reasoningContent *param.ReasoningContent
	if msg.ReasoningContent != nil {
		reasoningContent = &param.ReasoningContent{
			Content:   msg.ReasoningContent.Content,
			Signature: msg.ReasoningContent.Signature,
		}
	}
	msgParam := param.NewAssistantMessage(msg.Content, msg.GetToolCalls(), reasoningContent)
	logItem := session.LogItem{
		Id:      session.NewLogItemId(),
		Role:    param.RoleAssistant,
		Created: time.Now().Unix(),
		Message: &msgParam,
	}
	return c.appendMessage(inMsg.Channel, inMsg.ChatId, logItem, true)
}

// --- User message building ---

// buildUserLogItem constructs a LogItem from user input, handling attachments and media refs.
func (c *ContextManager) buildUserLogItem(inMsg *UserInput) (session.LogItem, error) {
	unionParamsWithKey, err := parseInputAttachments(inMsg.Attachments)
	if err != nil {
		return session.LogItem{}, err
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
				if refName, err := media.SaveMedia(xstring.ToBytes(un.ImageURL.URL), un.Key); err == nil {
					logItem.Metadata.ImageRef[idx] = refName
				}
			}
		}

		unionParams := make([]*param.ContentUnion, 0, len(unionParamsWithKey)+1)
		for _, un := range unionParamsWithKey {
			unionParams = append(unionParams, un.ContentUnion)
		}
		if inMsg.Content != "" {
			unionParams = append(unionParams, &param.ContentUnion{
				Text: &param.Text{Value: inMsg.Content},
			})
		}
		msgParam = param.NewUserMessage(unionParams)
	} else {
		msgParam = param.NewUserMessage(inMsg.Content)
	}

	logItem.Message = &msgParam
	return logItem, nil
}

// --- Context query ---

func (c *ContextManager) GetMessageContext(channel, chatId string) ([]param.Message, error) {
	log, err := c.contextLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return nil, err
	}

	logs := log.GetLogs()
	msgList := make([]param.Message, 0, len(logs)+1)
	msgList = append(msgList, param.NewSystemMessage(c.getRenderedSystemPrompts()))
	for _, item := range logs {
		msgList = append(msgList, *item.Message)
	}
	return msgList, nil
}

func (c *ContextManager) GetSystemPrompt() string {
	return c.getRenderedSystemPrompts()
}

func (c *ContextManager) GetMessageHistory(channel, chatId string) ([]session.LogItem, error) {
	return c.getAOFLogItems(channel, chatId)
}

// --- Context compaction ---

func (c *ContextManager) ClearSession(channel, chatId string) error {
	contextLog, err := c.contextLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return err
	}

	contextLog.ResetLogsFromMessage(nil)
	return contextLog.Flush(c.contextLogManager.Workspace)
}

func (c *ContextManager) CompressToolCalls(channel, chatId string, count int) (int, error) {
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
func (c *ContextManager) SummarizeHistory(
	ctx context.Context,
	channel, chatId string,
	llmFunc func(context.Context, []param.Message) (string, error),
) error {
	contextLog, err := c.contextLogManager.GetOrCreate(channel, chatId)
	if err != nil {
		return err
	}

	logs := contextLog.GetLogs()

	const recentKeepCount = 5
	if len(logs) < 10 || len(logs) <= recentKeepCount {
		return nil
	}

	startIdx, endIdx := adjustSummarizeBounds(logs, 0, len(logs)-recentKeepCount)
	if endIdx <= startIdx {
		return nil
	}

	toSummarize := make([]param.Message, 0, endIdx-startIdx)
	for i := startIdx; i < endIdx; i++ {
		toSummarize = append(toSummarize, *logs[i].Message)
	}

	summary, err := llmFunc(ctx, toSummarize)
	if err != nil {
		return fmt.Errorf("failed to summarize history: %w", err)
	}

	newMsgList := make([]param.Message, 0, 1+(len(logs)-endIdx))
	newMsgList = append(newMsgList, param.NewUserMessage(
		fmt.Sprintf("[Conversation History Summary]\n%s\n[End of Summary, Recent Messages Follow]", summary),
	))
	for i := endIdx; i < len(logs); i++ {
		newMsgList = append(newMsgList, *logs[i].Message)
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

type contentUnionWithKey struct {
	*param.ContentUnion
	Key string
}

func parseInputAttachments(attachments []*UserInputAttachment) ([]*contentUnionWithKey, error) {
	params := make([]*contentUnionWithKey, 0, len(attachments))
	for _, att := range attachments {
		switch att.Type {
		case ImageAttachment:
			params = append(params, &contentUnionWithKey{
				ContentUnion: &param.ContentUnion{
					ImageURL: &param.ImageURL{
						URL:       dataurl.Base64Encode(att.Data),
						MediaType: att.MimeType,
					},
				},
				Key: att.Key,
			})
		case FileAttachment:
			params = append(params, &contentUnionWithKey{
				ContentUnion: &param.ContentUnion{
					Text: &param.Text{Value: xstring.FromBytes(att.Data)},
				},
				Key: att.Key,
			})
		default:
			return nil, fmt.Errorf("unsupported attachment type: %s", att.Type)
		}
	}
	return params, nil
}
