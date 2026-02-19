package schema

type Choice struct {
	FinishReason FinishReason
	Index        int64
	Message      CompletionMessage
}

func (c *Choice) IsStopped() bool {
	return c.FinishReason == FinishReasonStop
}

func (c *Choice) IsLengthExceeded() bool {
	return c.FinishReason == FinishReasonLength
}

func (c *Choice) HasToolCalls() bool {
	return c.FinishReason == FinishReasonToolCalls
}

type StreamChoice struct {
	FinishReason FinishReason
	Index        int64
	Delta        StreamChoiceDelta
}

