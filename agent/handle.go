package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/ryanreadbooks/tokkibot/config"
	schema "github.com/ryanreadbooks/tokkibot/llm/schema"
)

// formatCancelledError formats the context cancellation error message
func formatCancelledError(ctx context.Context) string {
	return fmt.Sprintf("(operation cancelled by user: %s)", ctx.Err().Error())
}

// initMessageContext initializes session logs and appends the user message
func (a *Agent) initMessageContext(_ context.Context, userMsg *UserMessage) error {
	a.contextMgr.InitFromSessionLogs(userMsg.Channel, userMsg.ChatId)
	_, err := a.contextMgr.AppendUserMessage(userMsg)
	return err
}

func (a *Agent) handleIncomingMessage(ctx context.Context, userMsg *UserMessage) string {
	if err := a.initMessageContext(ctx, userMsg); err != nil {
		return err.Error()
	}

	agentCfg := config.GetAgentConfig()
	var lastResponse *schema.Response
	for curIter := 1; curIter <= agentCfg.MaxIteration; curIter++ {
		select {
		case <-ctx.Done():
			return formatCancelledError(ctx)
		default:
		}

		llmReq, err := a.buildLLMMessageRequest(ctx, userMsg)
		if err != nil {
			return fmt.Sprintf("(failed to build llm message request: %s)", err.Error())
		}

		llmResp, err := a.llm.ChatCompletion(ctx, llmReq)
		if err != nil {
			return fmt.Sprintf("(failed to call llm: %s)", err.Error())
		}
		lastResponse = llmResp

		choice := llmResp.FirstChoice()
		if err := a.contextMgr.AppendAssistantMessage(userMsg, &choice.Message); err != nil {
			return err.Error()
		}

		if choice.IsStopped() {
			return choice.Message.Content
		}

		if choice.HasToolCalls() {
			// IMPORTANT: must complete all tool calls to match assistant's tool_calls
			// even if ctx is cancelled, we need to save all tool results
			for _, tc := range choice.Message.ToolCalls {
				if err := a.handleToolCall(ctx, userMsg, &tc); err != nil {
					return err.Error()
				}
			}
		}
	}

	// max iterations reached
	if lastResponse != nil {
		return fmt.Sprintf("(max iterations reached, last response: %s)",
			lastResponse.FirstChoice().Message.Content)
	}
	return "(max iterations reached with no response)"
}

type toolCallAndResult struct {
	tc     schema.CompletionToolCall
	result string
}

// handle streaming tool call
//
// This method will be called when one tool call response is completed.
// Tool call will be invoked from another goroutine.
func (a *Agent) handleStreamingToolCall(dstTcs *[]*toolCallAndResult) schema.StreamToolCallHandler {
	dstTcsMu := sync.Mutex{}
	return func(ctx context.Context, tc schema.StreamChoiceDeltaToolCall) {
		// invoke tool
		result := a.getToolAndInvoke(ctx, &schema.CompletionToolCall{
			Id:       tc.Id,
			Type:     tc.Type,
			Function: tc.Function,
		})

		dstTcsMu.Lock()
		defer dstTcsMu.Unlock()
		*dstTcs = append(*dstTcs, &toolCallAndResult{
			tc: schema.CompletionToolCall{
				Id:       tc.Id,
				Type:     tc.Type,
				Function: tc.Function,
			},
			result: result,
		})
	}
}

func (a *Agent) handleIncomingMessageStream(ctx context.Context, userMsg *UserMessage, emitter StreamEmitter) {
	defer emitter.EmitDone()

	if err := a.initMessageContext(ctx, userMsg); err != nil {
		emitter.EmitContent(-1, err.Error(), "")
		return
	}

	agentCfg := config.GetAgentConfig()

mainLoop:
	for curIter := 1; curIter <= agentCfg.MaxIteration; curIter++ {
		select {
		case <-ctx.Done():
			emitter.EmitContent(curIter, formatCancelledError(ctx), "")
			break mainLoop
		default:
		}

		var (
			wg                      sync.WaitGroup
			contentBuilder          strings.Builder
			reasoningContentBuilder strings.Builder
			dstTcs                  = make([]*toolCallAndResult, 0)
		)

		llmReq, err := a.buildLLMMessageRequest(ctx, userMsg)
		if err != nil {
			emitter.EmitContent(curIter, err.Error(), "")
			break
		}
		// call llm the stream way
		llmRespCh := a.llm.ChatCompletionStream(ctx, llmReq)
		streamPacked := schema.StreamResponseHandler(
			ctx,
			llmRespCh,
			a.handleStreamingToolCall(&dstTcs),
		)

		wg.Go(func() {
			for content := range streamPacked.Content {
				emitter.EmitContent(curIter, content.Content, content.ReasoningContent)
				contentBuilder.WriteString(content.Content)
				reasoningContentBuilder.WriteString(content.ReasoningContent)
			}
		})

		wg.Go(func() {
			type toolCallAcc struct {
				Name string
				Args strings.Builder
			}
			toolCallsMap := make(map[string]*toolCallAcc)
			toolCallOrder := make([]string, 0)

			for toolCall := range streamPacked.ToolCall {
				tc, exists := toolCallsMap[toolCall.Id]
				if !exists {
					tc = &toolCallAcc{Name: toolCall.Name}
					tc.Args.WriteString(toolCall.ArgumentFragment)
					toolCallsMap[toolCall.Id] = tc
					toolCallOrder = append(toolCallOrder, toolCall.Id)
					emitter.EmitTool(curIter, toolCall.Name, "")
				} else {
					tc.Args.WriteString(toolCall.ArgumentFragment)
				}
			}

			// check if interrupted - if so, don't emit incomplete tool args
			select {
			case <-ctx.Done():
				return
			default:
			}

			for _, id := range toolCallOrder {
				tc := toolCallsMap[id]
				emitter.EmitTool(curIter, tc.Name, tc.Args.String())
			}
		})

		wg.Wait()

		assistantTcs := make([]schema.CompletionToolCall, 0, len(dstTcs))
		for _, tcr := range dstTcs {
			assistantTcs = append(assistantTcs, tcr.tc)
		}

		err = a.contextMgr.AppendAssistantMessage(userMsg, &schema.CompletionMessage{
			Content:          contentBuilder.String(),
			ToolCalls:        assistantTcs,
			ReasoningContent: reasoningContentBuilder.String(),
		})
		if err != nil {
			emitter.EmitContent(curIter, err.Error(), "")
			break
		}

		if len(dstTcs) == 0 {
			break
		}

		// IMPORTANT: must save all tool results to match assistant's tool_calls
		// even if ctx is cancelled, we need to complete the tool call sequence
		for _, tcr := range dstTcs {
			if err := a.contextMgr.AppendToolResult(userMsg, &tcr.tc, tcr.result); err != nil {
				emitter.EmitContent(curIter, err.Error(), "")
				break mainLoop
			}
		}
	}
}

func (a *Agent) handleToolCall(
	ctx context.Context,
	inMsg *UserMessage,
	tc *schema.CompletionToolCall,
) error {
	toolResult := a.getToolAndInvoke(ctx, tc)
	// feedback tool calling result to llm
	err := a.contextMgr.AppendToolResult(inMsg, tc, toolResult)
	return err
}

func (a *Agent) getToolAndInvoke(ctx context.Context, tc *schema.CompletionToolCall) string {
	select {
	case <-ctx.Done():
		return formatCancelledError(ctx)
	default:
	}

	a.toolsMu.RLock()
	tool, ok := a.tools[tc.Function.Name]
	if !ok {
		a.toolsMu.RUnlock()
		return fmt.Sprintf("(tool %s not found)", tc.Function.Name)
	}

	a.toolsMu.RUnlock()

	// execute tool
	toolResult, err := tool.Invoke(ctx, tc.Function.Arguments)
	if err != nil {
		toolResult = err.Error()
	}

	return toolResult
}
