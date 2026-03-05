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
	"github.com/ryanreadbooks/tokkibot/cron"
)

type gatewayOption struct {
	runCronTasks bool
	verbose      bool
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

type Gateway struct {
	agent    *agent.Agent
	wg       sync.WaitGroup
	adapters map[chmodel.Type]chadapter.Adapter
	poolMu   sync.Mutex
	pools    map[string]*ants.Pool

	// running tasks cancel functions, key: "channel:chatId"
	runningMu sync.RWMutex
	running   map[string]context.CancelFunc

	// cron manager
	cronMgr *cron.Manager

	verbose bool
	option  *gatewayOption
}

func NewGateway(ctx context.Context, opts ...GatewayOption) (*Gateway, error) {
	ag, err := agent.Prepare(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare agent: %w", err)
	}

	option := &gatewayOption{}
	for _, opt := range opts {
		opt(option)
	}

	pools := make(map[string]*ants.Pool)
	gateway := &Gateway{
		agent:    ag,
		pools:    pools,
		adapters: make(map[chmodel.Type]chadapter.Adapter),
		running:  make(map[string]context.CancelFunc),
		cronMgr:  cron.GetGlobalManager(),
		verbose:  option.verbose,
		option:   option,
	}

	// set cron task handler
	gateway.cronMgr.SetHandler(gateway.handleCronTask)

	// load cron tasks
	if err := gateway.cronMgr.Load(); err != nil {
		slog.Warn("failed to load cron tasks", "error", err)
	}

	return gateway, nil
}

func (g *Gateway) GetAgent() *agent.Agent {
	return g.agent
}

func (g *Gateway) AddAdapter(adapter chadapter.Adapter) {
	g.adapters[adapter.Type()] = adapter
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
	result := g.agent.Ask(ctx, userMessage)

	// send result back to adapter
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
	_ chadapter.Adapter,
) {
	emitter := &msgEmitter{msg: rawMsg}
	g.agent.AskStream(ctx, userMessage, emitter)
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

// handleCronTask handles a triggered cron task
func (g *Gateway) handleCronTask(ctx context.Context, task *cron.Task) {
	chatId := task.ChatId()

	// construct user message from cron task (use "cron" as channel for all cron tasks)
	userMessage := &agent.UserMessage{
		Channel: "cron",
		ChatId:  chatId,
		Content: task.Prompt(),
		Created: time.Now().Unix(),
	}

	// execute the agent
	result := g.agent.Ask(ctx, userMessage)
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
