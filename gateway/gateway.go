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
		case msg := <-adapter.ReceiveChan():
			slog.Info("received message", "adapter", adapter.Type(), "message", msg)

			g.poolMu.Lock()
			poolKey := fmt.Sprintf("%s:%s", msg.Channel, msg.ChatId)
			pool, ok := g.pools[poolKey]
			if !ok {
				pool, _ = ants.NewPool(1) // IMPORTANT: make sure one channel one chat is handled by one goroutine
				g.pools[poolKey] = pool
			}
			g.poolMu.Unlock()

			pool.Submit(func() {
				attachments := make([]*agent.UserMessageAttachment, 0, len(msg.Attachments))
				for _, attachment := range msg.Attachments {
					attachments = append(attachments, &agent.UserMessageAttachment{
						Key:  attachment.Key,
						Type: agent.AttachmentType(attachment.Type),
						Data: attachment.Data,
					})
				}
				result := g.agent.Ask(msg.Context(), &agent.UserMessage{
					Channel:     adapter.Type().String(),
					ChatId:      msg.ChatId,
					Content:     msg.Content,
					Created:     msg.Created,
					Attachments: attachments,
				})

				// send result back to adapter
				select {
				case adapter.SendChan() <- &chmodel.OutgoingMessage{
					SenderId: msg.SenderId,
					Channel:  msg.Channel,
					ChatId:   msg.ChatId,
					Content:  result,
					Metadata: msg.Metadata,
				}:
				default:
				}
			})
		}
	}
}
