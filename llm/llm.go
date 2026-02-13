package llm

import (
	"context"

	"github.com/ryanreadbooks/tokkibot/llm/model"
)

// The request sent to the LLM service.
type Request struct {
	Model    string
	Messages []model.MessageParam

	// -1 means using model defaults
	Temperature float64

	// -1 means using model defaults
	MaxTokens int64
	Tools     []model.ToolParam

	// number of responses to generate, default to 1
	N int64
}

func NewRequest(model string, messages []model.MessageParam) *Request {
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
	Choices     []model.Choice
	Usage       model.CompletionUsage
}

func (r *Response) FirstChoice() model.Choice {
	if len(r.Choices) == 0 {
		return model.Choice{
			FinishReason: model.FinishReasonStop,
			Message: model.CompletionMessage{
				Role:    model.RoleAssistant,
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
	Choices     []model.StreamChoice
	ServiceTier string
	Usage       model.CompletionUsage

	Err error // read err should be placed here
}

func (s *StreamResponseChunk) FirstChoice() model.StreamChoice {
	if len(s.Choices) == 0 {
		return model.StreamChoice{
			FinishReason: model.FinishReasonStop,
			Index:        0,
			Delta: model.StreamChoiceDelta{
				Role:    model.RoleAssistant,
				Content: "It seems the LLM service did not respond anything.",
			},
		}
	}

	return s.Choices[0]
}

type LLM interface {
	ChatCompletion(ctx context.Context, req *Request) (*Response, error)

	// You should read from the returned channel until it is closed.
	ChatCompletionStream(ctx context.Context, req *Request) <-chan *StreamResponseChunk
}
