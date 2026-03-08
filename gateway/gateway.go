package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/ryanreadbooks/tokkibot/agent"
	chadapter "github.com/ryanreadbooks/tokkibot/channel/adapter"
	chmodel "github.com/ryanreadbooks/tokkibot/channel/model"
	"github.com/ryanreadbooks/tokkibot/component/tool"
	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/cron"
)

type gatewayOption struct {
	runCronTasks bool
	verbose      bool
	agentNames   []string // agent names to initialize
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

type Gateway struct {
	agents   map[string]*agent.Agent
	wg       sync.WaitGroup
	adapters map[chmodel.Type]chadapter.Adapter
	// adapter -> agentName mapping
	adapterAgent map[chmodel.Type]string
	poolMu       sync.Mutex
	pools        map[string]*ants.Pool

	// running tasks cancel functions, key: "channel:chatId"
	runningMu sync.RWMutex
	running   map[string]context.CancelFunc

	// cron manager
	cronMgr *cron.Manager

	verbose bool
	option  *gatewayOption
}

func NewGateway(ctx context.Context, opts ...GatewayOption) (*Gateway, error) {
	option := &gatewayOption{}
	for _, opt := range opts {
		opt(option)
	}

	if len(option.agentNames) == 0 {
		// read all agent ids from config
		for _, entry := range config.GetConfig().Agents {
			option.agentNames = append(option.agentNames, entry.Id)
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
		agents[name] = ag
	}

	// prepare __crons agent (uses main's workspace, isolated sessions)
	cronsAg, err := agent.Prepare(ctx, config.CronsAgentName,
		agent.WithWorkspaceOverride(config.GetAgentWorkspaceDir(config.MainAgentName)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare crons agent: %w", err)
	}
	agents[config.CronsAgentName] = cronsAg

	gateway := &Gateway{
		agents:       agents,
		pools:        make(map[string]*ants.Pool),
		adapters:     make(map[chmodel.Type]chadapter.Adapter),
		adapterAgent: make(map[chmodel.Type]string),
		running:      make(map[string]context.CancelFunc),
		cronMgr:      cron.GetGlobalManager(),
		verbose:      option.verbose,
		option:       option,
	}

	gateway.cronMgr.SetHandler(gateway.handleCronTask)

	if err := gateway.cronMgr.Load(); err != nil {
		slog.Warn("failed to load cron tasks", "error", err)
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

// agentForAdapter returns the agent bound to the given adapter channel type.
func (g *Gateway) agentForAdapter(channel chmodel.Type) *agent.Agent {
	agentName, ok := g.adapterAgent[channel]
	if !ok {
		agentName = config.MainAgentName
	}
	return g.agents[agentName]
}

// AddAdapter registers an adapter with an optional agent name binding.
// If agentName is empty, defaults to "main".
func (g *Gateway) AddAdapter(adapter chadapter.Adapter, agentName ...string) {
	g.adapters[adapter.Type()] = adapter
	name := config.MainAgentName
	if len(agentName) > 0 && agentName[0] != "" {
		name = agentName[0]
	}
	g.adapterAgent[adapter.Type()] = name
}

func (g *Gateway) Run(ctx context.Context) error {
	// schedule and start cron tasks
	g.cronMgr.RegisterAll()
	if g.option.runCronTasks {
		g.cronMgr.Start()
		defer g.cronMgr.Stop()
	}

	for _, adapter := range g.adapters {
		if g.verbose {
			slog.Info(fmt.Sprintf("channel %s begin to run...", adapter.Type()))
		}
		g.wg.Go(func() {
			adapter.Start(ctx)
		})
		g.wg.Go(func() {
			g.messageWorker(ctx, adapter)
		})
	}

	g.wg.Wait()

	return nil
}

func (g *Gateway) messageWorker(ctx context.Context, adapter chadapter.Adapter) {
	for {
		select {
		case <-ctx.Done():
			if g.verbose {
				slog.Info("message worker stopped", "adapter", adapter.Type(), "reason", ctx.Err())
			}
			return
		case rawMsg := <-adapter.ReceiveChan():
			if g.verbose {
				slog.Info("received message", "adapter", adapter.Type(),
					"sender_id", rawMsg.SenderId,
					"message", rawMsg.Content,
					"chat_id", rawMsg.ChatId,
					"attachements", len(rawMsg.Attachments))
			}

			// check for control commands first
			if cmd := parseControlCommand(rawMsg.Content); g.handleControl(rawMsg, cmd) {
				continue
			}

			g.poolMu.Lock()
			poolKey := fmt.Sprintf("%s:%s", rawMsg.Channel, rawMsg.ChatId)
			pool, ok := g.pools[poolKey]
			if !ok {
				pool, _ = ants.NewPool(1) // IMPORTANT: make sure one channel one chat is handled by one goroutine
				g.pools[poolKey] = pool
			}
			g.poolMu.Unlock()

			attachments := extractAttachments(rawMsg)
			userMessage := &agent.UserMessage{
				Channel:     adapter.Type().String(),
				ChatId:      rawMsg.ChatId,
				Content:     rawMsg.Content,
				Created:     rawMsg.Created,
				Attachments: attachments,
			}

			// create cancellable context for this task
			taskCtx, taskCancel := context.WithCancel(rawMsg.Context())

			// store cancel function for /stop command
			g.runningMu.Lock()
			g.running[poolKey] = taskCancel
			g.runningMu.Unlock()

			pool.Submit(func() {
				defer func() {
					// cleanup cancel function when task completes
					g.runningMu.Lock()
					delete(g.running, poolKey)
					g.runningMu.Unlock()
				}()

				if rawMsg.Stream {
					g.workerDoStream(taskCtx, rawMsg, userMessage, adapter)
				} else {
					g.workerDo(taskCtx, rawMsg, userMessage, adapter)
				}
			})
		}
	}
}

func (g *Gateway) workerDo(
	ctx context.Context,
	rawMsg *chmodel.IncomingMessage,
	userMessage *agent.UserMessage,
	adapter chadapter.Adapter,
) {
	confirmHandler := NewConfirmHandler(rawMsg)
	ctx = tool.WithConfirmer(ctx, confirmHandler)

	ag := g.agentForAdapter(adapter.Type())
	result := ag.Ask(ctx, userMessage)

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
) {
	confirmHandler := NewConfirmHandler(rawMsg)
	ctx = tool.WithConfirmer(ctx, confirmHandler)

	ag := g.agentForAdapter(adapter.Type())
	emitter := &msgEmitter{msg: rawMsg}
	ag.AskStream(ctx, userMessage, emitter)
}

// msgEmitter adapts IncomingMessage to agent.StreamEmitter
type msgEmitter struct {
	msg *chmodel.IncomingMessage
}

func (e *msgEmitter) EmitContent(round int, content, reasoning string) {
	e.msg.EmitContent(&chmodel.StreamContent{
		Round:            round,
		Content:          content,
		ReasoningContent: reasoning,
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

// handleCronTask handles a triggered cron task using the __crons virtual agent
func (g *Gateway) handleCronTask(ctx context.Context, task *cron.Task) {
	chatId := task.ChatId()

	userMessage := &agent.UserMessage{
		Channel: "cron",
		ChatId:  chatId,
		Content: task.Prompt(),
		Created: time.Now().Unix(),
	}

	cronsAgent := g.agents[config.CronsAgentName]
	result := cronsAgent.Ask(ctx, userMessage)
	slog.Info("cron task executed", "name", task.Name)

	// deliver result if configured
	if !task.Deliver {
		return
	}

	adapter, ok := g.adapters[task.DeliverChannel]
	if !ok {
		slog.Error("adapter not found for cron task delivery", "name", task.Name, "channel", task.DeliverChannel)
		return
	}

	select {
	case adapter.SendChan() <- &chmodel.OutgoingMessage{
		ReceiverId: task.DeliverTo,
		Channel:    task.DeliverChannel,
		ChatId:     chatId,
		Content:    result,
	}:
		slog.Info("cron task result delivered", "name", task.Name, "to", task.DeliverTo)
	default:
		slog.Warn("failed to deliver cron task result", "name", task.Name)
	}
}
