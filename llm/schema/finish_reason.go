package schema

type FinishReason string

const (
	FinishReasonStop      FinishReason = "stop"
	FinishReasonLength    FinishReason = "length"
	FinishReasonToolCalls FinishReason = "tool_calls"
	FinishReasonRefusal   FinishReason = "refusal" // content_filter
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

func (f FinishReason) IsRefused() bool {
	return f == FinishReasonRefusal
}
