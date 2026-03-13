package agent

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	agcontext "github.com/ryanreadbooks/tokkibot/agent/context"
	"github.com/ryanreadbooks/tokkibot/agent/tools"
	"github.com/ryanreadbooks/tokkibot/workspace"

	"github.com/ryanreadbooks/tokkibot/component/skill"
	"github.com/ryanreadbooks/tokkibot/component/tool"
	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/llm"
	"github.com/ryanreadbooks/tokkibot/llm/schema"
	"github.com/ryanreadbooks/tokkibot/llm/schema/param"
)

//go:embed summary.md
var summaryPrompt string

type (
	UserMessage           = agcontext.UserInput
	UserMessageAttachment = agcontext.UserInputAttachment
	AttachmentType        = agcontext.AttachmentType
)

type AgentConfig struct {
	RootCtx context.Context

	// Agent name, determines workspace and session isolation.
	Name string

	// The provider to use.
	Provider string

	// The model to use.
	Model string

	// Max tool-call iterations per request.
	MaxIteration int

	ResumeSessionId string

	WorkspaceDir string // workspace directory
	SessionDir   string // where session and context logs are stored

	isSpawned              bool
	doNotAutoRegisterTools bool
	subagentPrompt         string
}

type EmittedReasoningContentMetadata struct {
	ThinkingEnabled bool
}

type EmittedContent struct {
	Round            int
	Content          string
	ReasoningContent string
	Metadata         EmittedReasoningContentMetadata
}

// StreamEmitter is the interface for emitting stream events
type StreamEmitter interface {
	EmitContent(content *EmittedContent)
	EmitTool(round int, name, args string)
	EmitDone()
}

type loopState struct {
	mu   sync.Mutex
	data map[string]struct{}

	fn func(channel, chatId string) string
}

func (s *loopState) setState(channel, chatId string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[s.fn(channel, chatId)] = struct{}{}
}

func (s *loopState) unsetState(channel, chatId string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, s.fn(channel, chatId))
}

func (s *loopState) isStateSet(channel, chatId string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.data[s.fn(channel, chatId)]
	return ok
}

type Agent struct {
	cfg AgentConfig
	// The LLM service to use.
	llm llm.LLM

	toolsMu sync.RWMutex
	tools   map[string]tool.Invoker

	// The context manager for the agent.
	contextManager *agcontext.ContextManager

	// skill loader
	skillLoader *skill.Loader

	cachedReqsMu sync.RWMutex
	cachedReqs   map[string]*schema.Request

	// mcp
	mcpLoaded  atomic.Bool
	mcpManager *tool.McpToolManager

	// subagents results storage
	subAgentResultsMu sync.Mutex
	subAgentResults   map[string]chan string // name -> result channel

	loopState *loopState
}

func NewAgent(
	llm llm.LLM,
	cfg AgentConfig,
) *Agent {
	slog.Info("[agent] creating new agent",
		slog.Bool("is_spawned", cfg.isSpawned),
		slog.String("name", cfg.Name),
		slog.String("provider", cfg.Provider),
		slog.String("model", cfg.Model),
		slog.Int("max_iteration", cfg.MaxIteration))

	agentWorkspace := cfg.WorkspaceDir
	if agentWorkspace == "" {
		agentWorkspace = config.GetAgentWorkspaceDir(cfg.Name) // default workspace
		cfg.WorkspaceDir = agentWorkspace
	}
	slog.Debug("[agent] workspace configured", slog.String("workspace", agentWorkspace))

	// Set default session directory if not provided
	sessionDir := cfg.SessionDir
	if sessionDir == "" {
		sessionDir = filepath.Join(agentWorkspace, "sessions")
	}
	slog.Debug("[agent] session directory configured", slog.String("session_dir", sessionDir))

	skillLoader := skill.NewLoader()
	if err := skillLoader.Init(agentWorkspace); err != nil {
		// do not exit here, just log the error
		slog.Error("[agent] failed to init skill loader", slog.Any("error", err))
	}

	contextManager, err := agcontext.NewContextManager(
		cfg.RootCtx,
		agcontext.ContextManagerConfig{
			AgentName:            cfg.Name,
			AgentWorkspace:       agentWorkspace,
			SessionDir:           sessionDir,
			SystemPromptTemplate: cfg.subagentPrompt,
		},
		skillLoader,
	)
	if err != nil {
		slog.Error("[agent] failed to create context manager, now exit", slog.Any("error", err))
		os.Exit(1)
	}

	// mcp manager
	mcpManager := tool.NewMcpToolManager()
	// try to init mcp manager
	mcpConfig, err := config.GetMcpConfig()
	mcpLoaded := false
	if err != nil {
		slog.Debug("[agent] mcp config not found, mcp tools disabled")
	} else {
		mcpLoaded = true
		mcpManager.Init(cfg.RootCtx, mcpConfig)
		slog.Info("[agent] mcp tools loaded", slog.Int("tools_count", len(mcpManager.ListTools())))
	}

	agent := &Agent{
		cfg:            cfg,
		tools:          make(map[string]tool.Invoker),
		contextManager: contextManager,
		skillLoader:    skillLoader,
		cachedReqs:     make(map[string]*schema.Request),
		llm:            llm,
		mcpManager:     mcpManager,
		loopState: &loopState{
			data: make(map[string]struct{}),
			fn: func(channel, chatId string) string {
				return channel + ":" + chatId
			},
		},
	}
	agent.mcpLoaded.Store(mcpLoaded)
	if !cfg.isSpawned {
		agent.subAgentResults = make(map[string]chan string)
	}

	if !cfg.doNotAutoRegisterTools {
		agent.registerMainTools(agentWorkspace)
		slog.Info("[agent] agent created successfully", slog.String("name", cfg.Name), slog.Int("builtin_tools", len(agent.tools)))
	}

	return agent
}

// register tools
func (a *Agent) registerMainTools(agentWorkspace string) {
	a.registerBasicTools(agentWorkspace)

	a.RegisterTool(tools.Cron())
	a.RegisterTool(tools.Subagent(a))
}

func (a *Agent) registerBasicTools(agentWorkspace string) {
	workspaceReadDir := workspace.GetAllowedReadPaths(agentWorkspace)
	workspaceWriteDir := workspace.GetAllowedWritePaths(agentWorkspace)

	a.RegisterTool(tools.ReadFile(append([]string{config.GetProjectDir()}, workspaceReadDir...)))
	a.RegisterTool(tools.WriteFile(append([]string{config.GetProjectDir()}, workspaceWriteDir...)))
	a.RegisterTool(tools.ListDir(append([]string{config.GetProjectDir()}, workspaceReadDir...)))
	a.RegisterTool(tools.EditFile(append([]string{config.GetProjectDir()}, workspaceWriteDir...)))
	a.RegisterTool(tools.LoadRef())
	a.RegisterTool(tools.Shell())
	a.RegisterTool(tools.UseSkill(a.skillLoader))
	a.RegisterTool(tools.WebFetch())
	a.RegisterTool(tools.TodoWrite())
}

func (a *Agent) RegisterTool(tool tool.Invoker) {
	if tool == nil {
		return
	}

	a.toolsMu.Lock()
	defer a.toolsMu.Unlock()

	if _, ok := a.tools[tool.Info().Name]; ok {
		slog.Warn("[agent] tool already registered", slog.String("tool_name", tool.Info().Name))
	} else {
		a.tools[tool.Info().Name] = tool
	}
}

func (a *Agent) Name() string {
	return a.cfg.Name
}

func (a *Agent) providerConfig() config.ProviderConfig {
	return config.GetConfig().Providers[a.cfg.Provider]
}

// Handling incoming message in a blocking way
func (a *Agent) Ask(ctx context.Context, msg *UserMessage) string {
	return a.handleIncomingMessage(ctx, msg)
}

// Handling incoming message in a streaming way
func (a *Agent) AskStream(ctx context.Context, msg *UserMessage, emitter StreamEmitter) {
	a.handleIncomingMessageStream(ctx, msg, emitter)
}

func (a *Agent) buildLLMMessageRequest(ctx context.Context, msg *UserMessage) (*schema.Request, error) {
	// Check context size and compact if needed
	if err := a.checkAndCompactContext(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to compact context: %w", err)
	}

	msgList, err := a.contextManager.GetMessageContext(msg.Channel, msg.ChatId)
	if err != nil {
		return nil, err
	}
	providerCfg := a.providerConfig()
	r := schema.NewRequest(a.cfg.Model, msgList)
	r.Temperature = providerCfg.Temperature
	r.MaxTokens = int64(providerCfg.MaxTokens)
	if providerCfg.HasThinkingSet() {
		if providerCfg.IsThinkingEnabled() {
			r.Thinking = schema.EnableThinking()
		} else {
			r.Thinking = schema.DisableThinking()
		}
	}
	r.Tools = a.buildLLMTools()

	a.cachedReqsMu.Lock()
	defer a.cachedReqsMu.Unlock()
	a.cachedReqs[msg.Channel+":"+msg.ChatId] = r

	return r, nil
}

// checkAndCompactContext checks current context size and applies compression if needed
func (a *Agent) checkAndCompactContext(ctx context.Context, msg *UserMessage) error {
	providerCfg := a.providerConfig()
	currentTokens := a.GetCurrentContextTokens(msg.Channel, msg.ChatId)

	contextCompactThreshold := providerCfg.GetContextCompactThreshold()
	if currentTokens < contextCompactThreshold {
		return nil
	}

	// Step 1: Try compressing tool calls to refs
	compressed, err := a.contextManager.CompressToolCalls(msg.Channel, msg.ChatId, providerCfg.ToolCallCompressThreshold)
	if err != nil {
		return fmt.Errorf("failed to compress tool calls: %w", err)
	}

	if compressed > 0 {
		currentTokens = a.GetCurrentContextTokens(msg.Channel, msg.ChatId)
	}

	// Step 2: If over 80% threshold after compression, summarize history
	contextSummarizeThreshold := providerCfg.GetContextSummarizeThreshold()
	if currentTokens >= contextSummarizeThreshold {
		err = a.contextManager.SummarizeHistory(ctx, msg.Channel, msg.ChatId, a.summarizeMessagesWithLLM)
		if err != nil {
			return fmt.Errorf("failed to summarize history: %w", err)
		}
	}

	return nil
}

// summarizeMessagesWithLLM uses LLM to create a summary of conversation messages
func (a *Agent) summarizeMessagesWithLLM(ctx context.Context, messages []param.Message) (string, error) {
	summaryMsg := []param.Message{
		param.NewSystemMessage(summaryPrompt),
		param.NewUserMessage("Please summarize the conversation history above:"),
	}
	summaryMsg = append(summaryMsg, messages...)

	providerCfg := a.providerConfig()
	req := schema.NewRequest(a.cfg.Model, summaryMsg)
	req.Temperature = providerCfg.Temperature
	req.MaxTokens = 2000

	resp, err := a.llm.ChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.FirstChoice().Message.Content, nil
}

func (a *Agent) buildLLMTools() []param.Tool {
	params := make([]param.Tool, 0, len(a.tools))
	for _, tool := range a.tools {
		params = append(params, param.NewToolWithSchema(
			tool.Info().Name,
			tool.Info().Description,
			*tool.Info().Schema,
		))
	}

	if a.mcpLoaded.Load() {
		// add mcp tools
		for _, mcpTool := range a.mcpManager.ListTools() {
			params = append(params, param.NewToolWithSchema(
				mcpTool.Info().Name,
				mcpTool.Info().Description,
				*mcpTool.Info().Schema,
			))
		}
	}

	return params
}

// ClearContext clears all messages in a session
func (a *Agent) ClearContext(channel, chatId string) error {
	// Clear cached request for token estimation
	cacheKey := channel + ":" + chatId
	a.cachedReqsMu.Lock()
	delete(a.cachedReqs, cacheKey)
	a.cachedReqsMu.Unlock()

	return a.contextManager.ClearSession(channel, chatId)
}

// CompactContext forces context compaction for a session
func (a *Agent) CompactContext(ctx context.Context, channel, chatId string) (int, error) {
	providerCfg := a.providerConfig()

	// Step 1: Compress tool calls
	compressed, err := a.contextManager.CompressToolCalls(channel,
		chatId,
		providerCfg.ToolCallCompressThreshold,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to compress tool calls: %w", err)
	}

	// Step 2: Summarize history
	err = a.contextManager.SummarizeHistory(ctx, channel, chatId, a.summarizeMessagesWithLLM)
	if err != nil {
		return compressed, fmt.Errorf("failed to summarize history: %w", err)
	}

	return compressed, nil
}
