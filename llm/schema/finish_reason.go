package schema

type FinishReason string

const (
	FinishReasonStop      FinishReason = "stop"
	FinishReasonLength    FinishReason = "length"
	FinishReasonToolCalls FinishReason = "tool_calls"
)

func (f FinishReason) IsToolCalls() bool {
	return f == FinishReasonToolCalls
}

func (f FinishReason) IsStopped() bool {
	return f == FinishReasonStop
}

func (f FinishReason) IsLengthed() bool {
	return f == FinishReasonLength
}
