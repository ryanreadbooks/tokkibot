package context

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	"USER.md",
	"TOOLS.md",
}

func renderPromptTemplate(agentWorkspace string, skillLoader *skill.Loader, s string) string {
	tmpl, err := template.New("prompts").Parse(s)
	if err != nil {
		panic(err)
	}

	now := time.Now()
	var bd strings.Builder
	err = tmpl.Execute(&bd, promptBuiltinInfo{
		Cwd:             config.GetProjectDir(),
		Workspace:       agentWorkspace,
		Now:             fmt.Sprintf("%s, %s", now.String(), now.Weekday().String()),
		Runtime:         runtime.GOOS,
		AvailableSkills: skill.SkillsAsPrompt(skillLoader.Skills()),
		DateWithTz:      now.Format("2006-01-02 MST -0700"),
	})
	if err != nil {
		panic(err)
	}

	return bd.String()
}

func loadSystemPromptTemplate(
	agentWorkspace string,
	systemPrompt string,
	memoryMgr *MemoryManager,
) (string, error) {
	const separator = "\n\n---\n\n"

	var prompts strings.Builder
	prompts.Grow(4096)

	if systemPrompt != "" {
		prompts.WriteString(systemPrompt)
	} else {
		for _, name := range systemPromptList {
			content, err := os.ReadFile(filepath.Join(agentWorkspace, name))
			if err != nil {
				// USER.md is optional for backward compatibility.
				if os.IsNotExist(err) && name == "USER.md" {
					continue
				}
				return "", err
			}
			prompts.Write(content)
			prompts.WriteString(separator)
		}
	}

	if memoryPrompt := memoryMgr.Load(); memoryPrompt != "" {
		prompts.WriteString("\n\n")
		prompts.WriteString(memoryPrompt)
	}

	return prompts.String(), nil
}

func buildToolResultLogItem(
	toolCall *schema.CompletionToolCall,
	result string,
) session.LogItem {
	msgParam := param.NewToolMessage(toolCall.Id, result)
	return session.LogItem{
		Id:      session.NewLogItemId(),
		Role:    param.RoleTool,
		Created: time.Now().Unix(),
		Message: &msgParam,
	}
}

func buildAssistantLogItem(msg *schema.CompletionMessage) session.LogItem {
	var reasoningContent *param.ReasoningContent
	if msg.ReasoningContent != nil {
		reasoningContent = &param.ReasoningContent{
			Content:   msg.ReasoningContent.Content,
			Signature: msg.ReasoningContent.Signature,
		}
	}

	msgParam := param.NewAssistantMessage(msg.Content, msg.GetToolCalls(), reasoningContent)
	return session.LogItem{
		Id:      session.NewLogItemId(),
		Role:    param.RoleAssistant,
		Created: time.Now().Unix(),
		Message: &msgParam,
	}
}

func buildUserLogItemFromInput(inMsg *UserInput) (session.LogItem, error) {
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

func buildMessageContextWithSystemPrompt(
	systemPrompt string,
	logs []session.LogItem,
) []param.Message {
	msgList := make([]param.Message, 0, len(logs)+1)
	msgList = append(msgList, param.NewSystemMessage(systemPrompt))
	for _, item := range logs {
		msgList = append(msgList, *item.Message)
	}
	return msgList
}

func summarizeHistoryMessages(
	ctx context.Context,
	logs []session.LogItem,
	llmFunc func(context.Context, []param.Message) (string, error),
) ([]param.Message, bool, error) {
	const recentKeepCount = 5
	if len(logs) < 10 || len(logs) <= recentKeepCount {
		return nil, false, nil
	}

	startIdx, endIdx := adjustSummarizeBounds(logs, 0, len(logs)-recentKeepCount)
	if endIdx <= startIdx {
		return nil, false, nil
	}

	toSummarize := make([]param.Message, 0, endIdx-startIdx)
	for i := startIdx; i < endIdx; i++ {
		toSummarize = append(toSummarize, *logs[i].Message)
	}

	summary, err := llmFunc(ctx, toSummarize)
	if err != nil {
		return nil, false, fmt.Errorf("failed to summarize history: %w", err)
	}

	newMsgList := make([]param.Message, 0, 1+(len(logs)-endIdx))
	newMsgList = append(newMsgList, param.NewUserMessage(
		fmt.Sprintf("[Conversation History Summary]\n%s\n[End of Summary, Recent Messages Follow]", summary),
	))
	for i := endIdx; i < len(logs); i++ {
		newMsgList = append(newMsgList, *logs[i].Message)
	}

	return newMsgList, true, nil
}

func messagesToSessionLogItems(messages []param.Message) []session.LogItem {
	newLogs := make([]session.LogItem, 0, len(messages))
	for _, p := range messages {
		msg := p
		newLogs = append(newLogs, session.LogItem{
			Id:      session.NewLogItemId(),
			Role:    p.Role(),
			Created: time.Now().Unix(),
			Message: &msg,
		})
	}
	return newLogs
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
