package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/panjf2000/ants/v2"
	"github.com/ryanreadbooks/tokkibot/agent"
	chadapter "github.com/ryanreadbooks/tokkibot/channel/adapter"
	"github.com/ryanreadbooks/tokkibot/channel/adapter/lark"
	chmodel "github.com/ryanreadbooks/tokkibot/channel/model"
	"github.com/ryanreadbooks/tokkibot/config"

	"github.com/spf13/cobra"
)

var GatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Start the tokkibot gateway.",
	Long:  "Start the tokkibot gateway.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGateway(cmd.Context())
	},
}

type Gateway struct {
	agent    *agent.Agent
	pool     *ants.Pool
	wg       sync.WaitGroup
	adapters map[chmodel.Type]chadapter.Adapter
}

func initGateway(ctx context.Context) (*Gateway, error) {
	ag, err := agent.Prepare(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare agent: %w", err)
	}

	pool, _ := ants.NewPool(256)
	gateway := &Gateway{
		agent:    ag,
		pool:     pool,
		adapters: make(map[chmodel.Type]chadapter.Adapter),
	}

	// add registered adapters
	lark := lark.NewAdapter(lark.LarkConfig{
		AppId:     config.GetConfig().Adapters.Lark.AppId,
		AppSecret: config.GetConfig().Adapters.Lark.AppSecret,
	})
	gateway.adapters[lark.Type()] = lark

	return gateway, nil
}

func runGateway(ctx context.Context) error {
	gateway, err := initGateway(ctx)
	if err != nil {
		return fmt.Errorf("failed to init gateway: %w", err)
	}

	return gateway.run(ctx)
}

func (g *Gateway) run(ctx context.Context) error {
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
			g.pool.Submit(func() {
				result := g.agent.Ask(ctx, &agent.UserMessage{
					Channel: adapter.Type().String(),
					ChatId:  msg.ChatId,
					Content: msg.Content,
					Created: msg.Created,
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
