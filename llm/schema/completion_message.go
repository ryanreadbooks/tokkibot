package schema

import "github.com/ryanreadbooks/tokkibot/llm/schema/param"

// completion message responsed from LLM service
type CompletionMessage struct {
	Content          string
	ReasoningContent *ReasoningContent
	ToolCalls        []CompletionToolCall

	// assistant message only
	Role Role
}

func (m *CompletionMessage) HasToolCalls() bool {
	return len(m.ToolCalls) > 0
}

func (m *CompletionMessage) GetToolCalls() []*param.ToolCall {
	params := make([]*param.ToolCall, 0, len(m.ToolCalls))
	for _, toolCall := range m.ToolCalls {
		params = append(params, toolCall.ToToolCall())
	}
	return params
}

type ReasoningContent struct {
	Content   string `json:"content,omitempty"`
	Signature string `json:"signature,omitempty"` // anthropic style
}

type StreamChoiceDelta struct {
	Content          string
	ReasoningContent string
	Signature        string // reasoning signature
	Role             Role
	ToolCalls        []StreamChoiceDeltaToolCall
}

type StreamChoiceDeltaToolCall struct {
	Index    int64
	Type     ToolCallType
	Id       string
	Function CompletionToolCallFunction
}

func (d *StreamChoiceDelta) HasToolCalls() bool {
	return len(d.ToolCalls) > 0
}
