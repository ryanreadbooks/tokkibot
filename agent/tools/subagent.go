package tools

import (
	"context"
	"fmt"

	"github.com/ryanreadbooks/tokkibot/agent/tools/description"
	"github.com/ryanreadbooks/tokkibot/component/tool"
)

type SubagentAction string

const (
	SubagentActionSpawn     SubagentAction = "spawn"
	SubagentActionGetResult SubagentAction = "get_result"
)

type WaitMode string

const (
	WaitModeAny WaitMode = "any" // return when any one completes
	WaitModeAll WaitMode = "all" // return when all complete
)

type SubagentInput struct {
	Action SubagentAction `json:"action" jsonschema:"description=Action to perform: 'spawn' to create a subagent, 'get_result' to retrieve results from background subagents,enum=spawn,enum=get_result"`

	// Fields for spawn action
	Task       string `json:"task,omitempty"       jsonschema:"description=The task for the subagent to handle (required for spawn)"`
	Name       string `json:"name,omitempty"       jsonschema:"description=For spawn: optional subagent name"`
	Background bool   `json:"background,omitempty" jsonschema:"description=If true, the subagent runs in background and you must call get_result later to retrieve results"`

	// Fields for get_result action
	GetNames []string `json:"get_names,omitempty" jsonschema:"description=Names of subagents to retrieve results from (required for get_result)"`
	WaitMode WaitMode `json:"wait_mode,omitempty" jsonschema:"description=Wait mode: 'any' returns when first completes, 'all' waits for all to complete. Default: all,enum=any,enum=all"`
}

type SubagentResultItem struct {
	Name   string `json:"name"`
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

type SubagentResult struct {
	Action    SubagentAction        `json:"action"`
	AgentName string                `json:"agent_name,omitempty"` // for spawn
	Result    string                `json:"result,omitempty"`     // for spawn or single message
	Results   []*SubagentResultItem `json:"results,omitempty"`    // for get_result with multiple names
}

type SubAgentManager interface {
	SpawnSubagent(ctx context.Context, meta tool.InvokeMeta, input *SubagentInput) (*SubagentResult, error)
	GetSubagentResults(ctx context.Context, meta tool.InvokeMeta, names []string, waitMode WaitMode) (*SubagentResult, error)
}

func Subagent(manager SubAgentManager) tool.Invoker {
	return tool.NewInvoker(tool.Info{
		Name:        ToolNameSubagent,
		Description: description.SubagentDescription,
	},
		func(ctx context.Context, meta tool.InvokeMeta, input *SubagentInput) (*SubagentResult, error) {
			switch input.Action {
			case SubagentActionSpawn:
				if input.Task == "" {
					return nil, fmt.Errorf("task is required for spawn action")
				}
				return manager.SpawnSubagent(ctx, meta, input)
			case SubagentActionGetResult:
				if len(input.GetNames) == 0 {
					return nil, fmt.Errorf("get_names is required for get_result action")
				}
				waitMode := input.WaitMode
				if waitMode == "" {
					waitMode = WaitModeAll
				}
				return manager.GetSubagentResults(ctx, meta, input.GetNames, waitMode)
			default:
				return nil, fmt.Errorf("unknown action: %s, must be 'spawn' or 'get_result'", input.Action)
			}
		})
}
