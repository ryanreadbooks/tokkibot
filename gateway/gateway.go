package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/panjf2000/ants/v2"
	"github.com/ryanreadbooks/tokkibot/agent"
	chadapter "github.com/ryanreadbooks/tokkibot/channel/adapter"
	chmodel "github.com/ryanreadbooks/tokkibot/channel/model"
	"github.com/ryanreadbooks/tokkibot/pkg/misc"
)

type Gateway struct {
	agent    *agent.Agent
	wg       sync.WaitGroup
	adapters map[chmodel.Type]chadapter.Adapter
	poolMu   sync.Mutex
	pools    map[string]*ants.Pool
}

func NewGateway(ctx context.Context) (*Gateway, error) {
	ag, err := agent.Prepare(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare agent: %w", err)
	}

	pools := make(map[string]*ants.Pool)
	gateway := &Gateway{
		agent:    ag,
		pools:    pools,
		adapters: make(map[chmodel.Type]chadapter.Adapter),
	}

	return gateway, nil
}

func (g *Gateway) GetAgent() *agent.Agent {
	return g.agent
}

func (g *Gateway) AddAdapter(adapter chadapter.Adapter) {
	g.adapters[adapter.Type()] = adapter
}

func StartGateway(ctx context.Context) error {
	gateway, err := NewGateway(ctx)
	if err != nil {
		return fmt.Errorf("failed to init gateway: %w", err)
	}

	return gateway.Run(ctx)
}

func (g *Gateway) Run(ctx context.Context) error {
	for _, adapter := range g.adapters {
		slog.Info(fmt.Sprintf("channel %s started.", adapter.Type()))
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
			slog.Info("message worker stopped", "adapter", adapter.Type(), "reason", ctx.Err())
			return
		case rawMsg := <-adapter.ReceiveChan():
			slog.Info("received message", "adapter", adapter.Type(), "message", rawMsg)

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

			pool.Submit(func() {
				if rawMsg.Stream {
					g.workerDoStream(rawMsg, userMessage, adapter)
				} else {
					g.workerDo(rawMsg, userMessage, adapter)
				}
			})
		}
	}
}

func (g *Gateway) workerDo(
	rawMsg *chmodel.IncomingMessage,
	userMessage *agent.UserMessage,
	adapter chadapter.Adapter,
) {
	result := g.agent.Ask(rawMsg.Context(), userMessage)

	// send result back to adapter
	select {
	case adapter.SendChan() <- &chmodel.OutgoingMessage{
		SenderId: rawMsg.SenderId,
		Channel:  rawMsg.Channel,
		ChatId:   rawMsg.ChatId,
		Content:  result,
		Metadata: rawMsg.Metadata,
	}:
	default:
	}
}

func (g *Gateway) workerDoStream(
	rawMsg *chmodel.IncomingMessage,
	userMessage *agent.UserMessage,
	_ chadapter.Adapter,
) {
	streamResult := g.agent.AskStream(rawMsg.Context(), userMessage)
	if rawMsg.StreamTool() == nil {
		misc.DiscardChan(streamResult.ToolCall)
	} else {
		// send to tool call output channel
		for tool := range streamResult.ToolCall {
			rawMsg.StreamTool() <- &chmodel.StreamTool{
				Name:      tool.Name,
				Arguments: tool.Arguments,
			}
		}
	}

	if rawMsg.StreamContent() == nil {
		misc.DiscardChan(streamResult.Content)
	} else {
		// send content to output channel
		for content := range streamResult.Content {
			rawMsg.StreamContent() <- &chmodel.StreamContent{
				Content:          content.Content,
				ReasoningContent: content.ReasoningContent,
			}
		}
	}
}
