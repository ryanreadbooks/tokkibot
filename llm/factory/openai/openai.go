package openai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ryanreadbooks/tokkibot/llm"
	"github.com/ryanreadbooks/tokkibot/llm/schema"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	openaiparam "github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/shared"
)

const (
	reasoningContentKey = "reasoning_content"
)

var _ llm.LLM = (*OpenAI)(nil)

type OpenAI struct {
	client *openai.Client
}

type Config struct {
	ApiKey  string
	BaseURL string
}

func New(config Config) (*OpenAI, error) {
	if config.ApiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}

	if config.BaseURL == "" {
		return nil, fmt.Errorf("api base is required")
	}

	cli := openai.NewClient(
		option.WithAPIKey(config.ApiKey),
		option.WithBaseURL(config.BaseURL),
	)

	return &OpenAI{
		client: &cli,
	}, nil
}

func toSystemMessageParamUnion(param *schema.SystemMessageParam) *openai.ChatCompletionSystemMessageParam {
	if param != nil {
		union := &openai.ChatCompletionSystemMessageParam{}
		if param.String != nil {
			union.Content.OfString = openaiparam.NewOpt(param.String.Value)
		}
		for _, text := range param.Text {
			if text != nil {
				union.Content.OfArrayOfContentParts = append(
					union.Content.OfArrayOfContentParts, openai.ChatCompletionContentPartTextParam{
						Text: text.Value,
					})
			}
		}

		return union
	}

	return nil
}

func toUserMessageParamUnion(param *schema.UserMessageParam) *openai.ChatCompletionUserMessageParam {
	if param != nil {
		union := &openai.ChatCompletionUserMessageParam{}
		if param.String != nil {
			union.Content.OfString = openaiparam.NewOpt(param.String.Value)
		}

		for _, contentPart := range param.ContentParts {
			if contentPart != nil {
				if contentPart.Text != nil {
					union.Content.OfArrayOfContentParts = append(
						union.Content.OfArrayOfContentParts,
						openai.ChatCompletionContentPartUnionParam{
							OfText: &openai.ChatCompletionContentPartTextParam{
								Text: contentPart.Text.Value,
							},
						},
					)
				}
				if contentPart.ImageURL != nil {
					union.Content.OfArrayOfContentParts = append(
						union.Content.OfArrayOfContentParts,
						openai.ChatCompletionContentPartUnionParam{
							OfImageURL: &openai.ChatCompletionContentPartImageParam{
								ImageURL: openai.ChatCompletionContentPartImageImageURLParam{
									URL: contentPart.ImageURL.URL,
								},
							},
						},
					)
				}
			}
		}

		return union
	}

	return nil
}

func toAssistantMessageParamUnion(param *schema.AssistantMessageParam) *openai.ChatCompletionAssistantMessageParam {
	if param != nil {
		union := &openai.ChatCompletionAssistantMessageParam{}
		if param.Content != nil {
			union.Content.OfString = openaiparam.NewOpt(param.Content.Value)
		}
		for _, text := range param.Texts {
			if text != nil {
				union.Content.OfArrayOfContentParts = append(
					union.Content.OfArrayOfContentParts,
					openai.ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion{
						OfText: &openai.ChatCompletionContentPartTextParam{
							Text: text.Value,
						},
					},
				)
			}
		}
		for _, tc := range param.ToolCalls {
			if tc != nil && tc.Function != nil {
				union.ToolCalls = append(
					union.ToolCalls,
					openai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							ID: tc.Function.Id,
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      tc.Function.Name,
								Arguments: tc.Function.Arguments,
							},
						},
					},
				)
			}
		}

		return union
	}

	return nil
}

func toToolMessageParamUnion(param *schema.ToolMessageParam) *openai.ChatCompletionToolMessageParam {
	if param != nil {
		union := &openai.ChatCompletionToolMessageParam{
			ToolCallID: param.ToolCallId,
		}
		if param.String != nil {
			union.Content.OfString = openaiparam.NewOpt(param.String.Value)
		}
		for _, text := range param.Texts {
			if text != nil {
				union.Content.OfArrayOfContentParts = append(union.Content.OfArrayOfContentParts,
					openai.ChatCompletionContentPartTextParam{
						Text: text.Value,
					})
			}
		}

		return union
	}

	return nil
}

func toChatCompletionMessageParamUnion(param *schema.MessageParam) openai.ChatCompletionMessageParamUnion {
	union := openai.ChatCompletionMessageParamUnion{
		OfSystem:    toSystemMessageParamUnion(param.SystemMessageParam),
		OfUser:      toUserMessageParamUnion(param.UserMessageParam),
		OfAssistant: toAssistantMessageParamUnion(param.AssistantMessageParam),
		OfTool:      toToolMessageParamUnion(param.ToolMessageParam),
	}

	return union
}

func toToolParamUnion(param *schema.ToolParam) openai.ChatCompletionToolUnionParam {
	tool := openai.ChatCompletionToolUnionParam{}
	if param.Definition != nil {
		tool.OfFunction = &openai.ChatCompletionFunctionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        param.Definition.Name,
				Description: openaiparam.NewOpt(param.Definition.Description),
				Parameters:  param.Parameters,
			},
		}
	}

	return tool
}

func toChatCompletionNewParams(req *schema.Request) (openai.ChatCompletionNewParams, []option.RequestOption) {
	params := openai.ChatCompletionNewParams{
		Model: req.Model,
		N:     openaiparam.NewOpt(max(1, req.N)),
		StreamOptions: openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: openaiparam.NewOpt(true),
		},
	}

	if req.Temperature != -1 {
		params.Temperature = openaiparam.NewOpt(req.Temperature)
	}
	if req.MaxTokens != -1 {
		params.MaxTokens = openaiparam.NewOpt(req.MaxTokens)
	}

	opts := []option.RequestOption{}

	// attach reasoning content if necessary in request body in json format
	// cuz this openai-sdk does not support setting reasoning_content in assistant message param
	for idx, message := range req.Messages {
		params.Messages = append(params.Messages, toChatCompletionMessageParamUnion(&message))
		if message.AssistantMessageParam != nil && message.AssistantMessageParam.ReasoningContent != nil {
			jsonKey := fmt.Sprintf("messages.%d.reasoning_content", idx)
			opts = append(opts, option.WithJSONSet(jsonKey, message.AssistantMessageParam.ReasoningContent.Value))
		}
	}

	for _, tool := range req.Tools {
		params.Tools = append(params.Tools, toToolParamUnion(&tool))
	}

	if req.Thinking != nil {
		opts = append(opts, option.WithJSONSet("thinking", req.Thinking))
	}

	return params, opts
}

func toChoice(choice openai.ChatCompletionChoice) schema.Choice {
	toolCalls := make([]schema.CompletionToolCall, 0, len(choice.Message.ToolCalls))
	for _, toolCall := range choice.Message.ToolCalls {
		toolCalls = append(toolCalls, schema.CompletionToolCall{
			Id:   toolCall.ID,
			Type: schema.ToolCallTypeFunction,
			Function: schema.CompletionToolCallFunction{
				Name:      toolCall.Function.Name,
				Arguments: toolCall.Function.Arguments,
			},
		})
	}

	reasoningContent := choice.Message.JSON.ExtraFields[reasoningContentKey]
	var rs string
	_ = json.Unmarshal([]byte(reasoningContent.Raw()), &rs)

	// for custom field not supported by official openai sdk
	return schema.Choice{
		FinishReason: schema.FinishReason(choice.FinishReason),
		Index:        choice.Index,
		Message: schema.CompletionMessage{
			Role:             schema.Role(choice.Message.Role),
			Content:          choice.Message.Content,
			ReasoningContent: rs,
			ToolCalls:        toolCalls,
		},
	}
}

func toChoices(choices []openai.ChatCompletionChoice) []schema.Choice {
	chs := make([]schema.Choice, 0, len(choices))

	for _, choice := range choices {
		chs = append(chs, toChoice(choice))
	}

	return chs
}

func (o *OpenAI) ChatCompletion(ctx context.Context, req *schema.Request) (*schema.Response, error) {
	params, opts := toChatCompletionNewParams(req)
	resp, err := o.client.Chat.Completions.New(ctx, params, opts...)
	if err != nil {
		return nil, fmt.Errorf("openai chat completion new: %w", err)
	}

	return &schema.Response{
		Id:          resp.ID,
		Object:      string(resp.Object.Default()),
		Created:     resp.Created,
		Model:       resp.Model,
		ServiceTier: string(resp.ServiceTier),
		Choices:     toChoices(resp.Choices),
		Usage: schema.CompletionUsage{
			CompletionTokens: resp.Usage.CompletionTokens,
			PromptTokens:     resp.Usage.PromptTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}, nil
}

func toStreamChoice(choice openai.ChatCompletionChunkChoice) schema.StreamChoice {
	toolCalls := make([]schema.StreamChoiceDeltaToolCall, 0, len(choice.Delta.ToolCalls))
	for _, tc := range choice.Delta.ToolCalls {
		toolCalls = append(toolCalls, schema.StreamChoiceDeltaToolCall{
			Index: tc.Index,
			Id:    tc.ID,
			Type:  schema.ToolCallType(tc.Type),
			Function: schema.CompletionToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}

	reasoningContent := choice.Delta.JSON.ExtraFields[reasoningContentKey]
	var rs string
	_ = json.Unmarshal([]byte(reasoningContent.Raw()), &rs)

	return schema.StreamChoice{
		FinishReason: schema.FinishReason(choice.FinishReason),
		Index:        choice.Index,
		Delta: schema.StreamChoiceDelta{
			Role:             schema.Role(choice.Delta.Role),
			Content:          choice.Delta.Content,
			ToolCalls:        toolCalls,
			ReasoningContent: rs,
		},
	}
}

func toStreamChoices(choices []openai.ChatCompletionChunkChoice) []schema.StreamChoice {
	chs := make([]schema.StreamChoice, 0, len(choices))
	for _, choice := range choices {
		chs = append(chs, toStreamChoice(choice))
	}
	return chs
}

func toStreamResponseChunk(cur openai.ChatCompletionChunk) *schema.StreamResponseChunk {
	chunk := schema.StreamResponseChunk{
		Id:          cur.ID,
		Created:     cur.Created,
		Model:       cur.Model,
		Object:      string(cur.Object.Default()),
		Choices:     toStreamChoices(cur.Choices),
		ServiceTier: string(cur.ServiceTier),
		Usage: schema.CompletionUsage{
			CompletionTokens: cur.Usage.CompletionTokens,
			PromptTokens:     cur.Usage.PromptTokens,
			TotalTokens:      cur.Usage.TotalTokens,
		},
	}
	return &chunk
}

func (o *OpenAI) ChatCompletionStream(ctx context.Context, req *schema.Request) <-chan *schema.StreamResponseChunk {
	params, opts := toChatCompletionNewParams(req)
	stream := o.client.Chat.Completions.NewStreaming(ctx, params, opts...)
	ch := make(chan *schema.StreamResponseChunk, 16) // this should be buffered

	go func() {
		defer func() {
			if p := recover(); p != nil {
				ch <- &schema.StreamResponseChunk{Err: fmt.Errorf("panic: %v", p)}
			}

			stream.Close()
			close(ch)
		}()

		// read in the background
		for stream.Next() {
			select {
			case ch <- toStreamResponseChunk(stream.Current()):
			case <-ctx.Done():
				return
			}
		}

		if stream.Err() != nil {
			ch <- &schema.StreamResponseChunk{Err: stream.Err()}
		}
	}()

	return ch
}
