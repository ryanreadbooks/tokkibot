package gateway

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/ryanreadbooks/tokkibot/agent"
	chmodel "github.com/ryanreadbooks/tokkibot/channel/model"
	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/pkg/safe"
)

const (
	minHeartbeatInterval = 3 * time.Minute
)

type heartbeatEntry struct {
	mu        sync.Mutex
	agentName string
	cfg       config.AgentHeartbeatConfig
	ticker    *time.Ticker
}

func (e *heartbeatEntry) getCfg() config.AgentHeartbeatConfig {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cfg
}

func (e *heartbeatEntry) setCfg(cfg config.AgentHeartbeatConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg = cfg
}

type HeartbeatManager struct {
	mu      sync.RWMutex
	entries map[string]*heartbeatEntry

	gateway *Gateway
}

func NewHeartbeatManager(gateway *Gateway, cfgs map[string]config.AgentHeartbeatConfig) *HeartbeatManager {
	// create crons
	entries := make(map[string]*heartbeatEntry, len(cfgs))
	for agentName, cfg := range cfgs {
		duration, err := time.ParseDuration(cfg.Every)
		if err != nil {
			slog.Warn("invalid heartbeat interval, skipped",
				slog.String("agent", agentName),
				slog.String("every", cfg.Every),
				slog.Any("error", err),
			)
			continue
		}
		if duration == 0 {
			continue
		}

		if duration < minHeartbeatInterval {
			slog.Info("heartbeat interval adjusted to minimum",
				slog.String("agent", agentName),
				slog.String("configured", cfg.Every),
				slog.String("effective", minHeartbeatInterval.String()),
			)
		}
		duration = max(duration, minHeartbeatInterval)
		entries[agentName] = &heartbeatEntry{
			agentName: agentName,
			cfg:       cfg,
			ticker:    time.NewTicker(duration),
		}
	}

	return &HeartbeatManager{
		entries: entries,
		gateway: gateway,
	}
}

// runs in a separate goroutine
func (m *HeartbeatManager) Start(ctx context.Context) {
	for _, entry := range m.entries {
		entry := entry
		safe.Go(func() {
			slog.InfoContext(ctx, "heartbeat manager started",
				slog.String("agent", entry.agentName),
				slog.String("target", entry.cfg.Target),
				slog.String("to", entry.cfg.To),
			)

			for {
				select {
				case <-ctx.Done():
					entry.ticker.Stop()
					return
				case <-entry.ticker.C: // wait for next tick
					m.HandleHeartbeat(ctx, entry)
				}
			}
		})
	}
}

func (m *HeartbeatManager) HandleHeartbeat(ctx context.Context, entry *heartbeatEntry) {
	curHeartbeatCfg := entry.getCfg()
	if curHeartbeatCfg.Target == "" || curHeartbeatCfg.To == "" || curHeartbeatCfg.Prompt == "" {
		return
	}

	agentName := entry.agentName
	targetChannel := chmodel.Type(curHeartbeatCfg.Target)
	bindingAccount := m.gateway.getAgentBindingAccount(agentName, targetChannel)
	slog.InfoContext(ctx, "heartbeat triggered",
		slog.String("agent", agentName),
		slog.String("target", curHeartbeatCfg.Target),
		slog.String("account", bindingAccount),
		slog.String("to", curHeartbeatCfg.To),
	)

	// trigger hearbeat
	// find the adapter in gateway and send the message
	adapter := m.gateway.getDeliveryAdapter(
		targetChannel,
		bindingAccount,
	)
	if adapter == nil {
		slog.WarnContext(ctx, "adapter not found for heartbeat",
			slog.String("agent", agentName),
			slog.String("target", curHeartbeatCfg.Target),
			slog.String("account", bindingAccount),
			slog.String("to", curHeartbeatCfg.To),
		)
		return
	}

	targetAgent := m.gateway.getAgent(agentName)
	if targetAgent == nil {
		slog.WarnContext(ctx, "target agent not found for heartbeat",
			slog.String("agent", agentName),
			slog.String("target", curHeartbeatCfg.Target),
			slog.String("account", bindingAccount),
			slog.String("to", curHeartbeatCfg.To),
		)
		return
	}

	startAt := time.Now()
	result := targetAgent.Ask(ctx, &agent.UserMessage{
		Channel: curHeartbeatCfg.Target,
		ChatId:  curHeartbeatCfg.To,
		Content: curHeartbeatCfg.Prompt,
		Created: time.Now().Unix(),
	})
	elapsed := time.Since(startAt)

	if err := ctx.Err(); err != nil {
		slog.WarnContext(ctx, "heartbeat ask canceled",
			slog.String("agent", agentName),
			slog.String("target", curHeartbeatCfg.Target),
			slog.String("account", bindingAccount),
			slog.String("to", curHeartbeatCfg.To),
			slog.Any("error", err),
			slog.Int64("elapsed_ms", elapsed.Milliseconds()),
		)
		return
	}

	if strings.HasPrefix(strings.TrimSpace(result), "HEARTBEAT_NOTHING") {
		slog.InfoContext(ctx, "heartbeat nothing to do",
			slog.String("agent", agentName),
			slog.String("target", curHeartbeatCfg.Target),
			slog.String("account", bindingAccount),
			slog.String("to", curHeartbeatCfg.To),
		)
		return
	}

	select {
	case adapter.SendChan() <- &chmodel.OutgoingMessage{
		ReceiverId: curHeartbeatCfg.To,
		Channel:    targetChannel,
		ChatId:     curHeartbeatCfg.To,
		Content:    result,
		Metadata:   nil,
	}:
	default:
		slog.WarnContext(ctx, "heartbeat dropped: adapter output channel is full",
			slog.String("agent", agentName),
			slog.String("target", curHeartbeatCfg.Target),
			slog.String("account", bindingAccount),
			slog.String("to", curHeartbeatCfg.To),
		)
	}
}

func (m *HeartbeatManager) Update(agentName string, cfg config.AgentHeartbeatConfig) {
	duration, err := time.ParseDuration(cfg.Every)
	if err != nil {
		return
	}
	duration = max(duration, minHeartbeatInterval)

	m.mu.Lock()
	defer m.mu.Unlock()
	entry := m.entries[agentName]
	if entry == nil {
		return
	}

	entry.setCfg(cfg)
	entry.ticker.Reset(duration)
}
