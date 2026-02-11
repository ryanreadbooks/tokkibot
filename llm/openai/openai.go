package openai

import (
	"context"
	"fmt"

	"github.com/ryanreadbooks/tokkibot/llm"
	"github.com/ryanreadbooks/tokkibot/llm/model"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	openaiparam "github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/shared"
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

func toSystemMessageParamUnion(param *model.SystemMessageParam) *openai.ChatCompletionSystemMessageParam {
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

func toUserMessageParamUnion(param *model.UserMessageParam) *openai.ChatCompletionUserMessageParam {
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

func toAssistantMessageParamUnion(param *model.AssistantMessageParam) *openai.ChatCompletionAssistantMessageParam {
	if param != nil {
		union := &openai.ChatCompletionAssistantMessageParam{}
		if param.String != nil {
			union.Content.OfString = openaiparam.NewOpt(param.String.Value)
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

func toToolMessageParamUnion(param *model.ToolMessageParam) *openai.ChatCompletionToolMessageParam {
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

func toChatCompletionMessageParamUnion(param *model.MessageParam) openai.ChatCompletionMessageParamUnion {
	union := openai.ChatCompletionMessageParamUnion{
		OfSystem:    toSystemMessageParamUnion(param.SystemMessageParam),
		OfUser:      toUserMessageParamUnion(param.UserMessageParam),
		OfAssistant: toAssistantMessageParamUnion(param.AssistantMessageParam),
		OfTool:      toToolMessageParamUnion(param.ToolMessageParam),
	}

	return union
}

func toToolParamUnion(param *model.ToolParam) openai.ChatCompletionToolUnionParam {
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

func toChatCompletionNewParams(req *llm.Request) openai.ChatCompletionNewParams {
	params := openai.ChatCompletionNewParams{
		Model:       req.Model,
		Temperature: openaiparam.NewOpt(req.Temperature),
		MaxTokens:   openaiparam.NewOpt(req.MaxTokens),
	}

	for _, message := range req.Messages {
		params.Messages = append(params.Messages, toChatCompletionMessageParamUnion(&message))
	}

	for _, tool := range req.Tools {
		params.Tools = append(params.Tools, toToolParamUnion(&tool))
	}

	return params
}

func toChoice(choice openai.ChatCompletionChoice) model.Choice {
	toolCalls := make([]model.CompletionToolCall, 0, len(choice.Message.ToolCalls))
	for _, toolCall := range choice.Message.ToolCalls {
		toolCalls = append(toolCalls, model.CompletionToolCall{
			Id:   toolCall.ID,
			Type: model.ToolCallTypeFunction,
			Function: model.CompletionToolCallFunction{
				Name:      toolCall.Function.Name,
				Arguments: toolCall.Function.Arguments,
			},
		})
	}

	return model.Choice{
		FinishReason: model.FinishReason(choice.FinishReason),
		Index:        choice.Index,
		Message: model.CompletionMessage{
			Role:      model.Role(choice.Message.Role),
			Content:   choice.Message.Content,
			ToolCalls: toolCalls,
		},
	}
}

func toChoices(choices []openai.ChatCompletionChoice) []model.Choice {
	chs := make([]model.Choice, 0, len(choices))

	for _, choice := range choices {
		chs = append(chs, toChoice(choice))
	}

	return chs
}

func (o *OpenAI) ChatCompletion(ctx context.Context, req *llm.Request) (*llm.Response, error) {
	params := toChatCompletionNewParams(req)
	resp, err := o.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("openai chat completion new: %w", err)
	}

	return &llm.Response{
		Id:          resp.ID,
		Object:      string(resp.Object.Default()),
		Created:     resp.Created,
		Model:       resp.Model,
		ServiceTier: string(resp.ServiceTier),
		Choices:     toChoices(resp.Choices),
		Usage: model.CompletionUsage{
			CompletionTokens: resp.Usage.CompletionTokens,
			PromptTokens:     resp.Usage.PromptTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}, nil
}

func (o *OpenAI) ChatCompletionStream(ctx context.Context, req *llm.Request) (<-chan *llm.Response, error) {
	params := toChatCompletionNewParams(req)
	stream := o.client.Chat.Completions.NewStreaming(ctx, params)

	_ = stream
	return nil, nil
}
