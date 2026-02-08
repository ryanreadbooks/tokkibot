package model

// completion message responsed from LLM service
type CompletionMessage struct {
	Content   string
	ToolCalls []CompletionToolCall

	// assistant message only
	Role Role
}

func (m *CompletionMessage) HasToolCalls() bool {
	return len(m.ToolCalls) > 0
}

func (m *CompletionMessage) GetToolCallParams() []*ToolCallParam {
	params := make([]*ToolCallParam, 0, len(m.ToolCalls))
	for _, toolCall := range m.ToolCalls {
		params = append(params, toolCall.ToToolCallParam())
	}
	return params
}
