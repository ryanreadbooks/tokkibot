package agent

import (
	"context"
	_ "embed"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	agenttool "github.com/ryanreadbooks/tokkibot/agent/tools"
	"github.com/ryanreadbooks/tokkibot/component/tool"
	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/pkg/safe"
)

//go:embed subagent.md
var subagentPrompt string

const (
	subAgentExecuteTimeout = 30 * time.Minute
	subAgentResultExpiry   = 10 * time.Minute
)

func (a *Agent) spawnSubAgent(ctx context.Context, meta tool.InvokeMeta, input *agenttool.SubagentInput) (*Agent, error) {
	subAgentName := fmt.Sprintf("subagent-%s-%s", meta.ChatId, uuid.NewString()[:8])
	if input.Name != "" {
		subAgentName = input.Name
	}

	subAgentCfg := AgentConfig{
		RootCtx:      a.cfg.RootCtx,
		Name:         subAgentName,
		Provider:     a.cfg.Provider,
		Model:        a.cfg.Model,
		MaxIteration: a.cfg.MaxIteration,
		WorkspaceDir: a.cfg.WorkspaceDir,
		SessionDir:   config.GetSubAgentSessionsDir(a.Name(), subAgentName),

		isSpawned:              true,
		doNotAutoRegisterTools: true,
		subagentPrompt:         subagentPrompt,
	}

	subAgent := NewAgent(a.llm, subAgentCfg)
	subAgent.registerBasicTools(a.cfg.WorkspaceDir)

	return subAgent, nil
}

func (a *Agent) SpawnSubagent(ctx context.Context, meta tool.InvokeMeta, input *agenttool.SubagentInput) (*agenttool.SubagentResult, error) {
	subAgent, err := a.spawnSubAgent(ctx, meta, input)
	if err != nil {
		return nil, fmt.Errorf("failed to spawn subagent: %w", err)
	}

	msg := &UserMessage{
		Channel: meta.Channel,
		ChatId:  meta.ChatId,
		Content: input.Task,
	}

	if input.Background {
		resultCh := make(chan string, 1)
		agentName := subAgent.Name()

		a.subAgentResultsMu.Lock()
		a.subAgentResults[agentName] = resultCh
		a.subAgentResultsMu.Unlock()

		// expiry timer to cleanup unclaimed results
		time.AfterFunc(subAgentResultExpiry, func() {
			a.subAgentResultsMu.Lock()
			delete(a.subAgentResults, agentName)
			a.subAgentResultsMu.Unlock()
		})

		safe.Go(func() {
			result := subAgent.Ask(ctx, msg)
			select {
			case resultCh <- result:
			default:
			}
		})

		return &agenttool.SubagentResult{
			Action:    agenttool.SubagentActionSpawn,
			AgentName: agentName,
			Result:    "Subagent is running in background. Call get_result with this name to retrieve the result.",
		}, nil
	}

	result := subAgent.Ask(ctx, msg)
	return &agenttool.SubagentResult{
		Action:    agenttool.SubagentActionSpawn,
		AgentName: subAgent.Name(),
		Result:    result,
	}, nil
}

func (a *Agent) GetSubagentResults(ctx context.Context, meta tool.InvokeMeta, names []string, waitMode agenttool.WaitMode) (*agenttool.SubagentResult, error) {
	a.subAgentResultsMu.Lock()
	channels := make(map[string]chan string)
	notFound := make([]string, 0)
	for _, name := range names {
		if ch, exists := a.subAgentResults[name]; exists {
			channels[name] = ch
		} else {
			notFound = append(notFound, name)
		}
	}
	a.subAgentResultsMu.Unlock()

	results := make([]*agenttool.SubagentResultItem, 0, len(names))

	for _, name := range notFound {
		results = append(results, &agenttool.SubagentResultItem{
			Name:  name,
			Error: "no background subagent found with this name",
		})
	}

	if len(channels) == 0 {
		return &agenttool.SubagentResult{
			Action:  agenttool.SubagentActionGetResult,
			Results: results,
		}, nil
	}

	if waitMode == agenttool.WaitModeAny {
		cases := make([]reflect.SelectCase, 0, len(channels)+1)
		names := make([]string, 0, len(channels))

		cases = append(cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ctx.Done()),
		})

		for name, ch := range channels {
			cases = append(cases, reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(ch),
			})
			names = append(names, name)
		}

		chosen, value, ok := reflect.Select(cases)
		if chosen == 0 {
			return nil, ctx.Err()
		}

		name := names[chosen-1]
		result := ""
		if ok {
			result = value.String()
		}

		results = append(results, &agenttool.SubagentResultItem{
			Name:   name,
			Result: result,
		})

		a.subAgentResultsMu.Lock()
		delete(a.subAgentResults, name)
		a.subAgentResultsMu.Unlock()

	} else {
		for name, ch := range channels {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case result := <-ch:
				results = append(results, &agenttool.SubagentResultItem{
					Name:   name,
					Result: result,
				})
			}
		}

		a.subAgentResultsMu.Lock()
		for name := range channels {
			delete(a.subAgentResults, name)
		}
		a.subAgentResultsMu.Unlock()
	}

	return &agenttool.SubagentResult{
		Action:  agenttool.SubagentActionGetResult,
		Results: results,
	}, nil
}

// receiveSubAgentResults collects completed results (non-blocking) for auto-push.
func (a *Agent) receiveSubAgentResults() string {
	a.subAgentResultsMu.Lock()
	defer a.subAgentResultsMu.Unlock()

	if len(a.subAgentResults) == 0 {
		return ""
	}

	var completed []struct {
		name   string
		result string
	}

	for name, ch := range a.subAgentResults {
		select {
		case result := <-ch:
			completed = append(completed, struct {
				name   string
				result string
			}{name: name, result: result})
		default:
		}
	}

	for _, c := range completed {
		delete(a.subAgentResults, c.name)
	}

	if len(completed) == 0 {
		return ""
	}

	var bd strings.Builder
	bd.Grow(512)
	for _, r := range completed {
		fmt.Fprintf(&bd, "<SubAgentResult>\n")
		fmt.Fprintf(&bd, "\t<Name>%s</Name>\n", r.name)
		fmt.Fprintf(&bd, "\t<Result>%s</Result>\n", r.result)
		fmt.Fprintf(&bd, "</SubAgentResult>\n")
	}

	return bd.String()
}
