package llm

import (
	"context"

	"github.com/ryanreadbooks/tokkibot/llm/model"
)

// The request sent to the LLM service.
type Request struct {
	Model       string
	Messages    []model.MessageParam
	Temperature float64
	MaxTokens   int64
	Tools       []model.ToolParam
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

type LLM interface {
	ChatCompletion(ctx context.Context, req *Request) (*Response, error)

	// TODO
	ChatCompletionStream(ctx context.Context, req *Request) (<-chan *Response, error)
}
