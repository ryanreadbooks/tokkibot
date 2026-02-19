package schema

// The request sent to the LLM service.
type Request struct {
	Model    string
	Messages []MessageParam

	// -1 means using model defaults
	Temperature float64

	// -1 means using model defaults
	MaxTokens int64
	Tools     []ToolParam

	// number of responses to generate, default to 1
	N int64

	Thinking *Thinking
}

func NewRequest(model string, messages []MessageParam) *Request {
	return &Request{
		Model:       model,
		Messages:    messages,
		N:           1,
		Temperature: -1,
		MaxTokens:   -1,
	}
}

// The response from the LLM service.
type Response struct {
	Id string
	// always "chat.completion"
	Object string
	// unix timestamp in seconds
	Created     int64
	Model       string
	ServiceTier string
	Choices     []Choice
	Usage       CompletionUsage
}

func (r *Response) FirstChoice() Choice {
	if len(r.Choices) == 0 {
		return Choice{
			FinishReason: FinishReasonStop,
			Message: CompletionMessage{
				Role:    RoleAssistant,
				Content: "It seems the LLM service did not respond anything.",
			},
		}
	}

	return r.Choices[0]
}

type StreamResponseChunk struct {
	Id          string
	Created     int64
	Model       string
	Object      string
	Choices     []StreamChoice
	ServiceTier string
	Usage       CompletionUsage

	Err error // read err should be placed here
}

func (s *StreamResponseChunk) FirstChoice() StreamChoice {
	if len(s.Choices) == 0 {
		return StreamChoice{
			FinishReason: FinishReasonStop,
			Index:        0,
			Delta: StreamChoiceDelta{
				Role:    RoleAssistant,
				Content: "It seems the LLM service did not respond anything.",
			},
		}
	}

	return s.Choices[0]
}
