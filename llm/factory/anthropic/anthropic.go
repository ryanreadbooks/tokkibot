package anthropic

import (
	"context"
	"fmt"
	"time"

	"github.com/ryanreadbooks/tokkibot/llm"
	"github.com/ryanreadbooks/tokkibot/llm/schema"
	"github.com/ryanreadbooks/tokkibot/llm/schema/param"
	"github.com/ryanreadbooks/tokkibot/pkg/xstring"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	sdkparam "github.com/anthropics/anthropic-sdk-go/packages/param"
)

type Anthropic struct {
	client *sdk.Client
}

var _ llm.LLM = (*Anthropic)(nil)

type Config struct {
	ApiKey  string
	BaseURL string
}

func New(config Config) (*Anthropic, error) {
	if config.ApiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}

	if config.BaseURL == "" {
		return nil, fmt.Errorf("api base is required")
	}

	client := sdk.NewClient(
		option.WithAPIKey(config.ApiKey),
		option.WithBaseURL(config.BaseURL),
	)

	return &Anthropic{
		client: &client,
	}, nil
}

func toToolUnionParams(tools []param.Tool) []sdk.ToolUnionParam {
	union := make([]sdk.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		union = append(union, sdk.ToolUnionParam{
			OfTool: &sdk.ToolParam{
				Name:        tool.Definition.Name,
				Description: sdkparam.NewOpt(tool.Definition.Description),
				InputSchema: sdk.ToolInputSchemaParam{
					Required:   tool.InputSchema.Required,
					Properties: tool.InputSchema.Properties,
				},
			},
		})
	}

	return union
}

func toMessageNewParams(req *schema.Request) sdk.MessageNewParams {
	newParams := sdk.MessageNewParams{
		MaxTokens: req.MaxTokens,
		Model:     sdk.Model(req.Model),
	}

	if newParams.MaxTokens <= 0 {
		// max tokens is required for anthropic
		newParams.MaxTokens = 8192
	}

	if req.Temperature != -1 {
		newParams.Temperature = sdkparam.NewOpt(req.Temperature)
	}

	newParams.Messages = toMessageParams(req.Messages)
	newParams.System = toSystemMessageParam(req.Messages)
	newParams.Tools = toToolUnionParams(req.Tools)

	return newParams
}

func toSystemMessageParam(msgs []param.Message) []sdk.TextBlockParam {
	system := make([]sdk.TextBlockParam, 0, len(msgs))
	// handle system prompt
	for _, reqMsg := range msgs {
		if reqMsg.Role() == schema.RoleSystem && reqMsg.System != nil {
			if reqMsg.System.String != nil {
				system = append(system, sdk.TextBlockParam{
					Text: reqMsg.System.String.GetValue(),
				})
			}

			for _, text := range reqMsg.System.Text {
				if text != nil {
					system = append(system, sdk.TextBlockParam{
						Text: text.GetValue(),
					})
				}
			}
		}
	}

	return system
}

func userMessageToContentBlockParamUnion(msg *param.UserMessage) []sdk.ContentBlockParamUnion {
	blocks := make([]sdk.ContentBlockParamUnion, 0, len(msg.ContentParts)+1)
	if val := msg.String.GetValue(); val != "" {
		blocks = append(blocks, sdk.NewTextBlock(val))
	}

	for _, part := range msg.ContentParts {
		if part.Text != nil {
			blocks = append(blocks, sdk.NewTextBlock(part.Text.GetValue()))
		}

		if part.ImageURL != nil && part.ImageURL.URL != "" {
			blocks = append(blocks, sdk.NewImageBlockBase64(part.ImageURL.MediaType, part.ImageURL.URL))
		}
	}

	return blocks
}

func assistantMessageToContentBlockParamUnion(msg *param.AssistantMessage) []sdk.ContentBlockParamUnion {
	blocks := make([]sdk.ContentBlockParamUnion, 0, len(msg.Texts)+2)
	if val := msg.Content.GetValue(); val != "" {
		blocks = append(blocks, sdk.NewTextBlock(val))
	}

	for _, text := range msg.Texts {
		if text != nil {
			blocks = append(blocks, sdk.NewTextBlock(text.GetValue()))
		}
	}

	for _, toolCall := range msg.ToolCalls {
		if toolCall != nil && toolCall.Function != nil {
			blocks = append(blocks, sdk.NewToolUseBlock(toolCall.Function.Id, toolCall.Function.Arguments, toolCall.Function.Name))
		}
	}

	// thinking content
	if msg.ReasoningContent != nil {
		blocks = append(blocks, sdk.NewThinkingBlock(msg.ReasoningContent.Signature, msg.ReasoningContent.Content))
	}

	return blocks
}

func toolMessageToContentBlockParamUnion(msg *param.ToolMessage) []sdk.ContentBlockParamUnion {
	var content string
	if msg.String != nil {
		content = msg.String.GetValue()
	} else {
		content = param.TextsContent(msg.Texts)
	}

	return []sdk.ContentBlockParamUnion{sdk.NewToolResultBlock(msg.ToolCallId, content, false)}
}

func toMessageParam(msg param.Message) sdk.MessageParam {
	mp := sdk.MessageParam{}

	switch msg.Role() {
	case param.RoleUser:
		mp.Role = sdk.MessageParamRoleUser
		mp.Content = userMessageToContentBlockParamUnion(msg.User)
	case param.RoleAssistant:
		mp.Role = sdk.MessageParamRoleAssistant
		mp.Content = assistantMessageToContentBlockParamUnion(msg.Assistant)
	case param.RoleTool: // tool call result
		// tool is user message
		mp.Role = sdk.MessageParamRoleUser
		mp.Content = toolMessageToContentBlockParamUnion(msg.Tool)
	}

	// ignore system
	return mp
}

func toMessageParams(req []param.Message) []sdk.MessageParam {
	msgs := make([]sdk.MessageParam, 0, len(req))
	for _, msg := range req {
		if msg.Role() == param.RoleSystem {
			continue
		}
		msgs = append(msgs, toMessageParam(msg))
	}

	return msgs
}

func getFinishReason(reason sdk.StopReason) schema.FinishReason {
	switch reason {
	case sdk.StopReasonEndTurn:
		return schema.FinishReasonStop
	case sdk.StopReasonMaxTokens:
		return schema.FinishReasonLength
	case sdk.StopReasonStopSequence:
		return schema.FinishReasonStop
	case sdk.StopReasonToolUse:
		return schema.FinishReasonToolCalls
	case sdk.StopReasonPauseTurn:
		return schema.FinishReasonStop
	}

	return schema.FinishReasonStop
}

func getCompletionMessage(blocks []sdk.ContentBlockUnion) schema.CompletionMessage {
	ret := schema.CompletionMessage{Role: param.RoleAssistant}
	for _, block := range blocks {
		switch block.Type {
		case "text":
			ret.Content = block.Text
		case "thinking":
			ret.ReasoningContent = &schema.ReasoningContent{
				Content:   block.Thinking,
				Signature: block.Signature,
			}
		case "tool_use":
			ret.ToolCalls = append(ret.ToolCalls, schema.CompletionToolCall{
				Id:   block.ID,
				Type: schema.ToolCallTypeFunction,
				Function: schema.CompletionToolCallFunction{
					Name:      block.Name,
					Arguments: xstring.FromBytes(block.Input),
				},
			})
		}
	}

	return ret
}

func getChoices(resp *sdk.Message) []schema.Choice {
	return []schema.Choice{
		{
			Index:        0,
			FinishReason: getFinishReason(resp.StopReason),
			Message:      getCompletionMessage(resp.Content),
		},
	}
}

func (a *Anthropic) ChatCompletion(ctx context.Context, req *schema.Request) (*schema.Response, error) {
	params := toMessageNewParams(req)
	resp, err := a.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic messages new: %w", err)
	}

	return &schema.Response{
		Id:          resp.ID,
		Object:      "chat.completion",
		Model:       req.Model,
		ServiceTier: string(resp.Usage.ServiceTier),
		Usage: schema.CompletionUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
		Choices: getChoices(resp),
	}, nil
}

func (a *Anthropic) ChatCompletionStream(ctx context.Context, req *schema.Request) <-chan *schema.StreamResponseChunk {
	params := toMessageNewParams(req)
	stream := a.client.Messages.NewStreaming(ctx, params)
	ch := make(chan *schema.StreamResponseChunk, 16) // this should be buffered

	go func() {
		defer func() {
			if p := recover(); p != nil {
				ch <- &schema.StreamResponseChunk{Err: fmt.Errorf("panic: %v", p)}
			}

			stream.Close()
			close(ch)
		}()

		state := &streamState{}

		for stream.Next() {
			chunk := toStreamResponseChunk(state, stream.Current())
			if len(chunk.Choices) == 0 {
				continue
			}

			select {
			case ch <- chunk:
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

type streamState struct {
	round        int64
	id           string
	model        string
	inputTokens  int64
	created      int64
	serviceTier  string
	outputTokens int64
	stopReason   sdk.StopReason
	signature    string

	// curToolUseId         string
	// curToolUseName       string
}

func toStreamResponseChunk(state *streamState, event sdk.MessageStreamEventUnion) *schema.StreamResponseChunk {
	choice := schema.StreamChoice{Delta: schema.StreamChoiceDelta{}}
	if state.round == 0 {
		choice.Delta.Role = schema.RoleAssistant
	}

	switch event := event.AsAny().(type) {
	case sdk.MessageStartEvent:
		state.id = event.Message.ID
		state.model = string(event.Message.Model)
		state.created = time.Now().Unix()
		state.inputTokens = event.Message.Usage.InputTokens
		state.serviceTier = string(event.Message.Usage.ServiceTier)
	case sdk.MessageDeltaEvent:
		state.stopReason = event.Delta.StopReason
		state.outputTokens = event.Usage.OutputTokens
	case sdk.ContentBlockStartEvent:
		switch block := event.ContentBlock.AsAny().(type) {
		case sdk.ToolUseBlock:
			choice.Delta.ToolCalls = append(choice.Delta.ToolCalls, schema.StreamChoiceDeltaToolCall{
				Index: event.Index,
				Id:    block.ID,
				Type:  schema.ToolCallTypeFunction,
				Function: schema.CompletionToolCallFunction{
					Name: block.Name,
					// first tool use input will be "{}" here, we ignore it
				},
			})
		}
	case sdk.ContentBlockDeltaEvent:
		switch delta := event.Delta.AsAny().(type) {
		case sdk.TextDelta:
			choice.Delta.Content = delta.Text
		case sdk.InputJSONDelta:
			// stream tool call handling
			if len(delta.PartialJSON) != 0 {
				choice.Delta.ToolCalls = append(choice.Delta.ToolCalls,
					schema.StreamChoiceDeltaToolCall{
						Index: event.Index,
						Type:  schema.ToolCallTypeFunction,
						Function: schema.CompletionToolCallFunction{
							Arguments: delta.PartialJSON,
						},
					})
			}
		case sdk.ThinkingDelta:
			choice.Delta.ReasoningContent = delta.Thinking
		case sdk.SignatureDelta:
			choice.Delta.Signature = delta.Signature
		}
	case sdk.ContentBlockStopEvent:
		// do nothing
	case sdk.MessageStopEvent:
		// do nothing
	}

	state.round++
	return &schema.StreamResponseChunk{
		Id:          state.id,
		Created:     state.created,
		Model:       state.model,
		Object:      "chat.completion.chunk",
		ServiceTier: state.serviceTier,
		Choices:     []schema.StreamChoice{choice},
		Usage: schema.CompletionUsage{
			PromptTokens:     state.inputTokens,
			CompletionTokens: state.outputTokens,
			TotalTokens:      state.inputTokens + state.outputTokens,
		},
	}
}
