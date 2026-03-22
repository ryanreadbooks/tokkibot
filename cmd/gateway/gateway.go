package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	chadapter "github.com/ryanreadbooks/tokkibot/channel/adapter"
	"github.com/ryanreadbooks/tokkibot/channel/adapter/lark"
	"github.com/ryanreadbooks/tokkibot/config"
	gw "github.com/ryanreadbooks/tokkibot/gateway"

	"github.com/spf13/cobra"
)

var GatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Start the tokkibot gateway.",
	Long:  "Start the tokkibot gateway.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return initAndRunGateway(cmd.Context())
	},
}

func initAndRunGateway(ctx context.Context) error {
	slog.Info("[cmd/gateway] initializing gateway")
	cfg := config.GetConfig()

	// check for heartbeat configs
	heartbeatCfgs := make(map[string]config.AgentHeartbeatConfig)
	for _, agentEntry := range cfg.Agents {
		if agentEntry.Heartbeat != nil {
			heartbeatCfgs[agentEntry.Name] = *agentEntry.Heartbeat
		}
	}
	g, err := gw.NewGateway(ctx,
		gw.WithVerbose(true),
		gw.WithRunCronTasks(true),
		gw.WithHeartbeatCfgs(heartbeatCfgs),
	)
	if err != nil {
		slog.Error("[cmd/gateway] failed to create gateway", slog.Any("error", err))
		return err
	}

	// auto-create adapters based on agent bindings in config
	// Reuse adapters for the same channel+account to avoid duplicate connections
	adapterCache := make(map[string]chadapter.Adapter) // key: "channel:account"
	for _, agentEntry := range cfg.Agents {
		if agentEntry.Binding == nil {
			continue
		}

		match := agentEntry.Binding.Match
		adapterKey := fmt.Sprintf("%s:%s", match.Channel, match.Account)

		adapter, exists := adapterCache[adapterKey]
		if !exists {
			var err error
			adapter, err = createAdapter(match.Channel, match.Account)
			if err != nil {
				slog.Warn("failed to create adapter for agent binding",
					slog.String("agent", agentEntry.Name),
					slog.String("channel", match.Channel),
					slog.String("account", match.Account),
					slog.Any("error", err))
				continue
			}
			adapterCache[adapterKey] = adapter
			slog.Info("adapter created from binding",
				slog.String("agent", agentEntry.Name),
				slog.String("channel", match.Channel),
				slog.String("account", match.Account))
		} else {
			slog.Info("adapter reused from binding",
				slog.String("agent", agentEntry.Name),
				slog.String("channel", match.Channel),
				slog.String("account", match.Account))
		}

		g.AddAdapterWithRouting(adapter, agentEntry.Name, match.Account, match.ChatIds)
	}

	return g.Run(ctx)
}

// createAdapter creates a channel adapter from config
func createAdapter(channelName, accountName string) (chadapter.Adapter, error) {
	raw, ok := config.GetChannelAccountRaw(channelName, accountName)
	if !ok {
		return nil, fmt.Errorf("channel %s account %s not found in config", channelName, accountName)
	}

	switch channelName {
	case "lark":
		var larkCfg lark.LarkConfig
		if err := json.Unmarshal(raw, &larkCfg); err != nil {
			return nil, fmt.Errorf("failed to parse lark config: %w", err)
		}
		return lark.NewAdapter(larkCfg), nil
	default:
		return nil, fmt.Errorf("unsupported channel type: %s", channelName)
	}
}
