package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/ryanreadbooks/tokkibot/agent"
	"github.com/ryanreadbooks/tokkibot/agent/tools"
	chadapter "github.com/ryanreadbooks/tokkibot/channel/adapter"
	chmodel "github.com/ryanreadbooks/tokkibot/channel/model"
	"github.com/ryanreadbooks/tokkibot/component/tool"
	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/cron"
	"github.com/ryanreadbooks/tokkibot/pkg/trace"
)

type gatewayOption struct {
	runCronTasks bool
	verbose      bool
	agentNames   []string // agent names to initialize

	enableAutoMessageDelivery bool
	enableCwdAccess           bool
}

type GatewayOption func(*gatewayOption)

func WithRunCronTasks(run bool) GatewayOption {
	return func(o *gatewayOption) {
		o.runCronTasks = run
	}
}

func WithVerbose(verbose bool) GatewayOption {
	return func(o *gatewayOption) {
		o.verbose = verbose
	}
}

// WithAgentNames specifies which agents to initialize.
// Defaults to ["main"] if not set.
func WithAgentNames(names []string) GatewayOption {
	return func(o *gatewayOption) {
		o.agentNames = names
	}
}

func WithDisableAutoMessageDelivery() GatewayOption {
	return func(o *gatewayOption) {
		o.enableAutoMessageDelivery = false
	}
}

func WithEnableCwdAccess(enable bool) GatewayOption {
	return func(o *gatewayOption) {
		o.enableCwdAccess = enable
	}
}

// routeRule defines how to route messages to an agent
type routeRule struct {
	agentName string
	chatIds   map[string]struct{} // empty means match all (fallback)
}

// adapterRouter manages routing for a single adapter
type adapterRouter struct {
	adapter chadapter.Adapter
	rules   []*routeRule
}

// matchAgent returns the agent name for a given chatId
func (r *adapterRouter) matchAgent(chatId string) string {
	var fallback string
	for _, rule := range r.rules {
		if len(rule.chatIds) == 0 {
			fallback = rule.agentName
			continue
		}
		if _, ok := rule.chatIds[chatId]; ok {
			return rule.agentName
		}
	}
	return fallback
}

type Gateway struct {
	agents  map[string]*agent.Agent
	wg      sync.WaitGroup
	routers []*adapterRouter
	// channel type → adapter (for cron delivery); keeps last-registered per type
	channelAdapters map[chmodel.Type]chadapter.Adapter
	poolMu          sync.Mutex
	pools           map[string]*ants.Pool

	// running tasks cancel functions, key: "agentName:channel:chatId"
	runningMu sync.RWMutex
	running   map[string]context.CancelFunc

	// cron manager
	cronMgr *cron.Manager

	verbose bool
	option  *gatewayOption
}

func NewGateway(ctx context.Context, opts ...GatewayOption) (*Gateway, error) {
	option := &gatewayOption{
		enableAutoMessageDelivery: true,
		enableCwdAccess:           false,
	}
	for _, opt := range opts {
		opt(option)
	}

	if len(option.agentNames) == 0 {
		// read all agent ids from config
		for _, entry := range config.GetConfig().Agents {
			option.agentNames = append(option.agentNames, entry.Name)
		}
	}
	if len(option.agentNames) == 0 {
		option.agentNames = []string{config.MainAgentName}
	}

	agents := make(map[string]*agent.Agent, len(option.agentNames))
	for _, name := range option.agentNames {
		ag, err := agent.Prepare(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare agent %s: %w", name, err)
		}
		if !option.enableAutoMessageDelivery {
			ag.UnRegisterTool(tools.ToolNameSendMessage)
		}
		agents[name] = ag
	}

	// prepare __cron agent (uses main's workspace, isolated sessions)
	cronsAg, err := agent.Prepare(ctx, config.CronsAgentName,
		agent.WithWorkspace(config.GetAgentWorkspaceDir(config.MainAgentName)),
		agent.WithSessionDir(config.GetCronSessionsDir()),
	)
	cronsAg.UnRegisterTool(tools.ToolNameSendMessage)

	if err != nil {
		return nil, fmt.Errorf("failed to prepare crons agent: %w", err)
	}
	agents[config.CronsAgentName] = cronsAg

	gateway := &Gateway{
		agents:          agents,
		pools:           make(map[string]*ants.Pool),
		channelAdapters: make(map[chmodel.Type]chadapter.Adapter),
		running:         make(map[string]context.CancelFunc),
		cronMgr:         cron.GetGlobalManager(),
		verbose:         option.verbose,
		option:          option,
	}

	gateway.cronMgr.SetHandler(gateway.handleCronTask)

	if err := gateway.cronMgr.Load(); err != nil {
		slog.Warn("failed to load cron tasks", slog.Any("error", err))
	}

	return gateway, nil
}

// GetAgent returns the agent for the given name. Returns main agent if name is empty.
func (g *Gateway) GetAgent(name ...string) *agent.Agent {
	agentName := config.MainAgentName
	if len(name) > 0 && name[0] != "" {
		agentName = name[0]
	}
	return g.agents[agentName]
}

func (g *Gateway) agentByName(name string) *agent.Agent {
	if ag, ok := g.agents[name]; ok {
		return ag
	}
	return g.agents[config.MainAgentName]
}

// AddAdapter registers an adapter with an optional agent name binding.
// This is a simplified API; all messages go to the specified agent.
func (g *Gateway) AddAdapter(adapter chadapter.Adapter, agentName ...string) {
	name := config.MainAgentName
	if len(agentName) > 0 && agentName[0] != "" {
		name = agentName[0]
	}
	g.AddAdapterWithRouting(adapter, name, nil)
}

// AddAdapterWithRouting registers an adapter with routing rules.
// If chatIds is nil/empty, all messages go to the specified agent.
// Multiple calls with the same adapter will add additional routing rules.
func (g *Gateway) AddAdapterWithRouting(adapter chadapter.Adapter, agentName string, chatIds []string) {
	// Find existing router for this adapter
	var router *adapterRouter
	for _, r := range g.routers {
		if r.adapter == adapter {
			router = r
			break
		}
	}

	// Create new router if not found
	if router == nil {
		router = &adapterRouter{adapter: adapter}
		g.routers = append(g.routers, router)
	}

	// Build chatId set
	chatIdSet := make(map[string]struct{})
	for _, id := range chatIds {
		chatIdSet[id] = struct{}{}
	}

	// Add routing rule
	router.rules = append(router.rules, &routeRule{
		agentName: agentName,
		chatIds:   chatIdSet,
	})

	g.channelAdapters[adapter.Type()] = adapter
}

func (g *Gateway) Run(ctx context.Context) error {
	// schedule and start cron tasks
	g.cronMgr.RegisterAll()
	if g.option.runCronTasks {
		g.cronMgr.Start()
		defer g.cronMgr.Stop()
	}

	for _, router := range g.routers {
		if g.verbose {
			var agentNames []string
			for _, r := range router.rules {
				agentNames = append(agentNames, r.agentName)
			}
			slog.Info(fmt.Sprintf("channel %s (agents: %v) begin to run...", router.adapter.Type(), agentNames))
		}
		g.wg.Go(func() {
			router.adapter.Start(ctx)
		})
		g.wg.Go(func() {
			g.messageWorker(ctx, router)
		})
	}

	g.wg.Wait()

	return nil
}

func (g *Gateway) messageWorker(ctx context.Context, router *adapterRouter) {
	adapter := router.adapter

	for {
		select {
		case <-ctx.Done():
			if g.verbose {
				slog.InfoContext(ctx, "message worker stopped",
					slog.String("adapter", adapter.Type().String()),
					slog.Any("reason", ctx.Err()))
			}
			return
		case rawMsg := <-adapter.ReceiveChan():
			// Route message to the appropriate agent
			agentName := router.matchAgent(rawMsg.ChatId)
			if agentName == "" {
				slog.WarnContext(ctx, "no agent matched for message, skipping",
					slog.String("channel", rawMsg.Channel.String()),
					slog.String("chat_id", rawMsg.ChatId))
				continue
			}

			messageId := ""
			if mid, ok := rawMsg.Metadata["message_id"].(string); ok {
				messageId = mid
			}
			traceInfo := trace.NewTraceInfo(
				rawMsg.Channel.String(),
				rawMsg.ChatId,
				messageId,
			)
			taskCtx := trace.WithTrace(rawMsg.Context(), traceInfo)

			if g.verbose {
				slog.InfoContext(taskCtx, "received message",
					slog.String("sender_id", rawMsg.SenderId),
					slog.String("agent", agentName),
					slog.Int("content_len", len(rawMsg.Content)),
					slog.Int("attachments", len(rawMsg.Attachments)))
			}

			if cmd := parseControlCommand(rawMsg.Content); g.handleControl(rawMsg, cmd, agentName) {
				continue
			}

			// Pool per agent:channel:chatId (size=1): serializes messages within
			// the same chat, while different chats and agents run in parallel.
			sessionKey := fmt.Sprintf("%s:%s", agentName, rawMsg.Key())
			chatPool := g.getOrCreatePool(sessionKey)

			attachments := extractAttachments(rawMsg)
			userMessage := &agent.UserMessage{
				Channel:     adapter.Type().String(),
				ChatId:      rawMsg.ChatId,
				Content:     rawMsg.Content,
				Created:     rawMsg.Created,
				Attachments: attachments,
			}

			taskCtx, taskCancel := context.WithCancel(taskCtx)
			runningKey := sessionKey

			g.runningMu.Lock()
			g.running[runningKey] = taskCancel
			g.runningMu.Unlock()

			go chatPool.Submit(func() {
				defer func() {
					g.runningMu.Lock()
					delete(g.running, runningKey)
					g.runningMu.Unlock()
				}()

				if rawMsg.Stream {
					g.workerDoStream(taskCtx, rawMsg, userMessage, adapter, agentName)
				} else {
					g.workerDo(taskCtx, rawMsg, userMessage, adapter, agentName)
				}
			})
		}
	}
}

func (g *Gateway) getOrCreatePool(name string) *ants.Pool {
	g.poolMu.Lock()
	defer g.poolMu.Unlock()

	pool, ok := g.pools[name]
	if !ok {
		pool, _ = ants.NewPool(1)
		g.pools[name] = pool
	}
	return pool
}

func (g *Gateway) workerDo(
	ctx context.Context,
	rawMsg *chmodel.IncomingMessage,
	userMessage *agent.UserMessage,
	adapter chadapter.Adapter,
	agentName string,
) {
	confirmHandler := NewConfirmHandler(rawMsg)
	ctx = tool.WithConfirmer(ctx, confirmHandler)

	ag := g.agentByName(agentName)
	askOpts := []agent.AskOption{}
	if g.option.enableAutoMessageDelivery {
		askOpts = append(askOpts, agent.WithMessageChannel(&agent.AskTemporaryMessageChannel{
			OutChan:  adapter.SendChan(),
			Metadata: rawMsg.Metadata,
		}))
	}
	result := ag.Ask(ctx, userMessage, askOpts...)

	select {
	case adapter.SendChan() <- &chmodel.OutgoingMessage{
		ReceiverId: rawMsg.SenderId,
		Channel:    rawMsg.Channel,
		ChatId:     rawMsg.ChatId,
		Content:    result,
		Metadata:   rawMsg.Metadata,
	}:
	default:
	}
}

func (g *Gateway) workerDoStream(
	ctx context.Context,
	rawMsg *chmodel.IncomingMessage,
	userMessage *agent.UserMessage,
	adapter chadapter.Adapter,
	agentName string,
) {
	confirmHandler := NewConfirmHandler(rawMsg)
	ctx = tool.WithConfirmer(ctx, confirmHandler)

	ag := g.agentByName(agentName)
	emitter := &msgEmitter{msg: rawMsg}
	askOpts := []agent.AskOption{}
	if g.option.enableAutoMessageDelivery {
		askOpts = append(askOpts, agent.WithMessageChannel(&agent.AskTemporaryMessageChannel{
			OutChan:  adapter.SendChan(),
			Metadata: rawMsg.Metadata,
		}))
	}
	ag.AskStream(ctx, userMessage, emitter, askOpts...)
}

// msgEmitter adapts IncomingMessage to agent.StreamEmitter
type msgEmitter struct {
	msg *chmodel.IncomingMessage
}

func (e *msgEmitter) EmitContent(content *agent.EmittedContent) {
	e.msg.EmitContent(&chmodel.StreamContent{
		Round:            content.Round,
		Content:          content.Content,
		ReasoningContent: content.ReasoningContent,
		ThinkingEnabled:  content.Metadata.ThinkingEnabled,
	})
}

func (e *msgEmitter) EmitTool(round int, name, args string) {
	e.msg.EmitTool(&chmodel.StreamTool{
		Round:     round,
		Name:      name,
		Arguments: args,
	})
}

func (e *msgEmitter) EmitDone() {
	e.msg.EmitDone()
}

// handleCronTask handles a triggered cron task using the __cron virtual agent
func (g *Gateway) handleCronTask(ctx context.Context, task *cron.Task) {
	chatId := task.ChatId()

	// Create trace info for cron task
	traceInfo := trace.NewTraceInfo("cron", chatId, "")
	ctx = trace.WithTrace(ctx, traceInfo)

	userMessage := &agent.UserMessage{
		Channel: "cron",
		ChatId:  chatId,
		Content: task.Prompt(),
		Created: time.Now().Unix(),
	}

	cronsAgent := g.agents[config.CronsAgentName]
	result := cronsAgent.Ask(ctx, userMessage)
	slog.InfoContext(ctx, "cron task executed", slog.String("name", task.Name))

	// deliver result if configured
	if !task.Deliver {
		return
	}

	adapter, ok := g.channelAdapters[task.DeliverChannel]
	if !ok {
		slog.ErrorContext(ctx, "adapter not found for cron task delivery",
			slog.String("name", task.Name),
			slog.String("channel", task.DeliverChannel.String()),
			slog.String("to", task.DeliverTo),
		)
		return
	}

	select {
	case adapter.SendChan() <- &chmodel.OutgoingMessage{
		ReceiverId: task.DeliverTo,
		Channel:    task.DeliverChannel,
		ChatId:     chatId,
		Content:    result,
	}:
		slog.InfoContext(ctx, "cron task result delivered",
			slog.String("name", task.Name),
			slog.String("channel", task.DeliverChannel.String()),
			slog.String("to", task.DeliverTo),
		)
	default:
		slog.WarnContext(ctx, "failed to deliver cron task result", slog.String("name", task.Name))
	}
}
