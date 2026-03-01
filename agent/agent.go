package agent

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"sync"

	agcontext "github.com/ryanreadbooks/tokkibot/agent/context"
	"github.com/ryanreadbooks/tokkibot/agent/tools"

	"github.com/ryanreadbooks/tokkibot/component/skill"
	"github.com/ryanreadbooks/tokkibot/component/tool"
	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/llm"
	schema "github.com/ryanreadbooks/tokkibot/llm/schema"
)

//go:embed summary_prompt.md
var summaryPrompt string

type (
	UserMessage           = agcontext.UserInput
	UserMessageAttachment = agcontext.UserInputAttachment
	AttachmentType        = agcontext.AttachmentType
)

type AgentConfig struct {
	RootCtx context.Context

	// The provider to use.
	Provider string

	// The model to use.
	Model string

	ResumeSessionId string
}

type Agent struct {
	c AgentConfig
	// The LLM service to use.
	llm llm.LLM

	toolsMu sync.RWMutex
	tools   map[string]tool.Invoker

	// The context manager for the agent.
	contextMgr *agcontext.ContextManager

	// skill loader
	skillLoader *skill.Loader

	cachedReqsMu sync.RWMutex
	cachedReqs   map[string]*schema.Request
}

func NewAgent(
	llm llm.LLM,
	c AgentConfig,
) *Agent {
	skillLoader := skill.NewLoader()
	if err := skillLoader.Init(); err != nil {
		slog.Error("[agent] failed to init skill loader, now exit", "error", err)
		os.Exit(1)
	}

	contextMgr, err := agcontext.NewContextManager(
		c.RootCtx,
		agcontext.ContextManagerConfig{
			Workspace: config.GetWorkspaceDir(),
		},
		skillLoader,
	)
	if err != nil {
		slog.Error("[agent] failed to create context manager, now exit", "error", err)
		os.Exit(1)
	}

	agent := &Agent{
		c:           c,
		tools:       make(map[string]tool.Invoker),
		contextMgr:  contextMgr,
		skillLoader: skillLoader,
		cachedReqs:  make(map[string]*schema.Request),
		llm:         llm,
	}

	agent.registerTools()

	return agent
}

// register tools
func (a *Agent) registerTools() {
	allowDirs := []string{config.GetProjectDir(), config.GetWorkspaceDir()}
	a.RegisterTool(tools.ReadFile(allowDirs))
	a.RegisterTool(tools.WriteFile(allowDirs))
	a.RegisterTool(tools.ListDir(allowDirs))
	a.RegisterTool(tools.EditFile(allowDirs))
	a.RegisterTool(tools.LoadRef())
	a.RegisterTool(tools.Shell())
	a.RegisterTool(tools.UseSkill(a.skillLoader))
	a.RegisterTool(tools.WebFetch())
}

func (a *Agent) RegisterTool(tool tool.Invoker) {
	if tool == nil {
		return
	}

	a.toolsMu.Lock()
	defer a.toolsMu.Unlock()

	if _, ok := a.tools[tool.Info().Name]; ok {
		slog.Warn("[agent] tool already registered", "tool_name", tool.Info().Name)
	} else {
		a.tools[tool.Info().Name] = tool
	}
}

// Handling incoming message in a blocking way
func (a *Agent) Ask(ctx context.Context, msg *UserMessage) string {
	return a.handleIncomingMessage(ctx, msg)
}

type AskStreamResultToolCall struct {
	Round     int
	Name      string
	Arguments string
}

type AskStreamResultContent struct {
	Round            int
	Content          string
	ReasoningContent string
}

type AskStreamResult struct {
	Content  chan *AskStreamResultContent
	ToolCall chan *AskStreamResultToolCall
}

// Handling incoming message in a streaming way
func (a *Agent) AskStream(ctx context.Context, msg *UserMessage) *AskStreamResult {
	res := AskStreamResult{
		Content:  make(chan *AskStreamResultContent, 16),
		ToolCall: make(chan *AskStreamResultToolCall, 16),
	}
	go a.handleIncomingMessageStream(ctx, msg, &res)

	return &res
}

func (a *Agent) buildLLMMessageRequest(ctx context.Context, msg *UserMessage) (*schema.Request, error) {
	// Check context size and compact if needed
	if err := a.checkAndCompactContext(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to compact context: %w", err)
	}

	msgList, err := a.contextMgr.GetMessageContext(msg.Channel, msg.ChatId)
	if err != nil {
		return nil, err
	}
	providerCfg := config.GetConfig().Providers[a.c.Provider]
	r := schema.NewRequest(a.c.Model, msgList)
	r.Temperature = providerCfg.Temperature
	r.MaxTokens = int64(providerCfg.MaxTokens)
	if providerCfg.IsThinkingEnabled() {
		r.Thinking = schema.EnableThinking()
	} else {
		r.Thinking = schema.DisableThinking()
	}
	r.Tools = a.buildLLMToolParams()

	a.cachedReqsMu.Lock()
	defer a.cachedReqsMu.Unlock()
	a.cachedReqs[msg.Channel+":"+msg.ChatId] = r

	return r, nil
}

// checkAndCompactContext checks current context size and applies compression if needed
func (a *Agent) checkAndCompactContext(ctx context.Context, msg *UserMessage) error {
	providerCfg := config.GetConfig().Providers[a.c.Provider]
	currentTokens := a.GetCurrentContextTokens(msg.Channel, msg.ChatId)

	contextCompactThreshold := providerCfg.GetContextCompactThreshold()
	if currentTokens < contextCompactThreshold {
		return nil
	}

	// Step 1: Try compressing tool calls to refs
	compressed, err := a.contextMgr.CompressToolCalls(msg.Channel, msg.ChatId, providerCfg.ToolCallCompressThreshold)
	if err != nil {
		return fmt.Errorf("failed to compress tool calls: %w", err)
	}

	if compressed > 0 {
		currentTokens = a.GetCurrentContextTokens(msg.Channel, msg.ChatId)
	}

	// Step 2: If over 80% threshold after compression, summarize history
	contextSummarizeThreshold := providerCfg.GetContextSummarizeThreshold()
	if currentTokens >= contextSummarizeThreshold {
		err = a.contextMgr.SummarizeHistory(ctx, msg.Channel, msg.ChatId, a.summarizeMessagesWithLLM)
		if err != nil {
			return fmt.Errorf("failed to summarize history: %w", err)
		}
	}

	return nil
}

// summarizeMessagesWithLLM uses LLM to create a summary of conversation messages
func (a *Agent) summarizeMessagesWithLLM(ctx context.Context, messages []schema.MessageParam) (string, error) {
	summaryMsg := []schema.MessageParam{
		schema.NewSystemMessageParam(summaryPrompt),
		schema.NewUserMessageParam("Please summarize the conversation history above:"),
	}
	summaryMsg = append(summaryMsg, messages...)

	providerCfg := config.GetConfig().Providers[a.c.Provider]
	req := schema.NewRequest(a.c.Model, summaryMsg)
	req.Temperature = providerCfg.Temperature
	req.MaxTokens = 2000

	resp, err := a.llm.ChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.FirstChoice().Message.Content, nil
}

func (a *Agent) buildLLMToolParams() []schema.ToolParam {
	params := make([]schema.ToolParam, 0, len(a.tools))
	for _, tool := range a.tools {
		params = append(params, schema.NewToolParamWithSchemaParam(
			tool.Info().Name,
			tool.Info().Description,
			*tool.Info().Schema,
		))
	}

	return params
}
