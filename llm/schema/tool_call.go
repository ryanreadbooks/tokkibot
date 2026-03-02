package schema

import "github.com/ryanreadbooks/tokkibot/llm/schema/param"

type ToolCallType string

const (
	ToolCallTypeFunction ToolCallType = "function"
)

type CompletionToolCall struct {
	Id       string                     `json:"id"       mapstructure:"id"`
	Type     ToolCallType               `json:"type"     mapstructure:"type"`
	Function CompletionToolCallFunction `json:"function" mapstructure:"function"`
}

func (t *CompletionToolCall) ToToolCall() *param.ToolCall {
	return &param.ToolCall{
		Function: &param.ToolCallFunction{
			Id:        t.Id,
			Name:      t.Function.Name,
			Arguments: t.Function.Arguments,
		},
	}
}

type CompletionToolCallFunction struct {
	// the arguments to call the function with
	Arguments string `json:"arguments"`

	// the name of the function to call
	Name string `json:"name"`
}

func GatherStreamTools(
	m map[int64]StreamChoiceDeltaToolCall, // index -> tool call mapping
	cur StreamChoiceDeltaToolCall,
) map[int64]StreamChoiceDeltaToolCall {
	curIdx := cur.Index

	if existing, ok := m[curIdx]; ok {
		oldArgs := existing.Function.Arguments
		newArgs := oldArgs + cur.Function.Arguments
		// update by replacing
		existing.Function.Arguments = newArgs
		m[curIdx] = existing
	} else {
		// this is a new tool call
		m[curIdx] = cur
	}

	return m
}
