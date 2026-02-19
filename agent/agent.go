package agent

import (
	"context"
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

type UserMessage = agcontext.UserInput

const (
	maxIterAllowed = 30

	modelTemperature = 1.0
	maxTokenAllowed  = 25000
)

type AgentConfig struct {
	RootCtx context.Context

	// The model to use.
	Model string

	ResumeSessionId string
}

type ThinkingState string

const (
	ThinkingStateThinking ThinkingState = "thinking"
	ThinkingStateDone     ThinkingState = "done"
)

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
		llm: llm,
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

func (a *Agent) AskStream(ctx context.Context, msg *UserMessage) *AskStreamResult {
	res := AskStreamResult{
		Content:  make(chan *AskStreamResultContent, 16),
		ToolCall: make(chan *AskStreamResultToolCall, 16),
	}
	go a.handleIncomingMessageStream(ctx, msg, &res)

	return &res
}

func (a *Agent) buildLLMMessageRequest(ctx context.Context, msg *UserMessage) (*schema.Request, error) {
	msgList, err := a.contextMgr.GetMessageContext(msg.Channel, msg.ChatId)
	if err != nil {
		return nil, err
	}
	r := schema.NewRequest(a.c.Model, msgList)
	r.Temperature = modelTemperature
	r.MaxTokens = maxTokenAllowed
	r.Thinking = schema.EnableThinking()
	r.Tools = a.buildLLMToolParams()

	a.cachedReqsMu.Lock()
	defer a.cachedReqsMu.Unlock()
	a.cachedReqs[msg.Channel+":"+msg.ChatId] = r

	return r, nil
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
