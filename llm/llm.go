package llm

import (
	"context"
	"sort"

	"github.com/ryanreadbooks/tokkibot/llm/model"
	"github.com/ryanreadbooks/tokkibot/pkg/xmap"
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

func SyncWaitStreamResponse(ch <-chan *StreamResponseChunk) ([]model.StreamChoice, error) {
	// choice index -> choice
	choicesMap := make(map[int64]model.StreamChoice)

	// choice index -> tool call index -> tool call
	choicesToolCallsMap := make(map[int64]map[int64]model.StreamChoiceDeltaToolCall)
	for chunk := range ch {
		if chunk.Err != nil {
			return nil, chunk.Err
		}

		for _, choice := range chunk.Choices {
			curIdx := choice.Index
			if existing, ok := choicesMap[curIdx]; ok {
				existing.Delta.Content += choice.Delta.Content
				if choice.FinishReason != "" {
					existing.FinishReason = model.FinishReason(choice.FinishReason)
				}
				choicesMap[curIdx] = existing
			} else {
				choicesMap[curIdx] = choice
			}

			if choice.Delta.HasToolCalls() {
				for _, toolCall := range choice.Delta.ToolCalls {
					if existing, ok := choicesToolCallsMap[curIdx]; ok {
						if existingToolCall, ok := existing[toolCall.Index]; ok {
							existingToolCall.Function.Arguments += toolCall.Function.Arguments
							choicesToolCallsMap[curIdx][toolCall.Index] = existingToolCall
						} else {
							choicesToolCallsMap[curIdx][toolCall.Index] = toolCall
						}
					} else {
						choicesToolCallsMap[curIdx] = make(map[int64]model.StreamChoiceDeltaToolCall)
						choicesToolCallsMap[curIdx][toolCall.Index] = toolCall
					}
				}
			}
		}
	}

	// assign tool calls to corresponding choices
	for idx, choice := range choicesMap {
		if toolCalls, ok := choicesToolCallsMap[choice.Index]; ok {
			tcs := xmap.Values(toolCalls)
			old := choicesMap[idx]
			old.Delta.ToolCalls = tcs

			// sort tool calls by index
			sort.Slice(old.Delta.ToolCalls, func(i, j int) bool {
				return old.Delta.ToolCalls[i].Index < old.Delta.ToolCalls[j].Index
			})
			choicesMap[idx] = old
		}
	}

	choices := xmap.Values(choicesMap)
	sort.Slice(choices, func(i, j int) bool {
		return choices[i].Index < choices[j].Index
	})

	return choices, nil
}
