package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	schema "github.com/ryanreadbooks/tokkibot/llm/schema"
)

// initMessageContext initializes session logs and appends the user message
func (a *Agent) initMessageContext(ctx context.Context, inMsg *UserMessage) error {
	a.contextMgr.InitFromSessionLogs(inMsg.Channel, inMsg.ChatId)
	_, err := a.contextMgr.AppendUserMessage(inMsg)
	return err
}

func (a *Agent) handleIncomingMessage(ctx context.Context, inMsg *UserMessage) string {
	if err := a.initMessageContext(ctx, inMsg); err != nil {
		return err.Error()
	}

	var lastResponse *schema.Response
	for curIter := 1; curIter <= maxIterAllowed; curIter++ {
		llmReq, err := a.buildLLMMessageRequest(ctx, inMsg)
		if err != nil {
			return fmt.Sprintf("(failed to build llm message request: %s)", err.Error())
		}

		llmResp, err := a.llm.ChatCompletion(ctx, llmReq)
		if err != nil {
			return fmt.Sprintf("(failed to call llm: %s)", err.Error())
		}
		lastResponse = llmResp

		choice := llmResp.FirstChoice()
		if err := a.contextMgr.AppendAssistantMessage(inMsg, &choice.Message); err != nil {
			return err.Error()
		}

		if choice.IsStopped() {
			return choice.Message.Content
		}

		if choice.HasToolCalls() {
			for _, tc := range choice.Message.ToolCalls {
				if err := a.handleToolCall(ctx, inMsg, &tc); err != nil {
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

func (a *Agent) handleIncomingMessageStream(
	ctx context.Context,
	inMsg *UserMessage,
	result *AskStreamResult,
) {
	if err := a.initMessageContext(ctx, inMsg); err != nil {
		result.Content <- &AskStreamResultContent{
			Round:   -1,
			Content: err.Error(),
		}
		close(result.ToolCall)
		close(result.Content)
		return
	}

mainLoop:
	for curIter := 1; curIter <= maxIterAllowed; curIter++ {
		var (
			wg                      sync.WaitGroup
			contentBuilder          strings.Builder
			reasoningContentBuilder strings.Builder
			dstTcs                  = make([]*toolCallAndResult, 0)
		)

		llmReq, err := a.buildLLMMessageRequest(ctx, inMsg)
		if err != nil {
			result.Content <- &AskStreamResultContent{
				Round:   curIter,
				Content: err.Error(),
			}
			break
		}
		// call llm the stream way
		llmRespCh := a.llm.ChatCompletionStream(ctx, llmReq)
		streamPacked := schema.StreamResponseHandler(ctx,
			llmRespCh, a.handleStreamingToolCall(&dstTcs))

		wg.Go(func() {
			for content := range streamPacked.Content {
				result.Content <- &AskStreamResultContent{
					Round:            curIter,
					Content:          content.Content,
					ReasoningContent: content.ReasoningContent,
				}
				contentBuilder.WriteString(content.Content)
				reasoningContentBuilder.WriteString(content.ReasoningContent)
			}
		})

		wg.Go(func() {
			// Track tool calls by ID to accumulate arguments
			// Use ID as unique identifier to handle multiple calls to same tool
			toolCallsMap := make(map[string]*AskStreamResultToolCall)
			toolCallOrder := make([]string, 0) // preserve order (store IDs)

			for toolCall := range streamPacked.ToolCall {
				tc, exists := toolCallsMap[toolCall.Id]
				if !exists {
					// New tool call - send initial notification
					tc = &AskStreamResultToolCall{
						Round:     curIter,
						Name:      toolCall.Name,
						Arguments: toolCall.ArgumentFragment,
					}
					toolCallsMap[toolCall.Id] = tc
					toolCallOrder = append(toolCallOrder, toolCall.Id)
					
					// Send initial notification (name only, empty args)
					result.ToolCall <- &AskStreamResultToolCall{
						Round:     curIter,
						Name:      toolCall.Name,
						Arguments: "", // empty = collecting
					}
				} else {
					// Accumulate arguments for existing tool call
					tc.Arguments += toolCall.ArgumentFragment
				}
			}

			// Send complete tool calls with full arguments in order
			for _, id := range toolCallOrder {
				tc := toolCallsMap[id]
				result.ToolCall <- &AskStreamResultToolCall{
					Round:     curIter,
					Name:      tc.Name,
					Arguments: tc.Arguments,
				}
			}
		})

		wg.Wait()

		assistantTcs := make([]schema.CompletionToolCall, 0, len(dstTcs))
		for _, tcr := range dstTcs {
			assistantTcs = append(assistantTcs, tcr.tc)
		}

		// accumulate assistant message for this iteration
		err = a.contextMgr.AppendAssistantMessage(inMsg, &schema.CompletionMessage{
			Content:          contentBuilder.String(),
			ToolCalls:        assistantTcs,
			ReasoningContent: reasoningContentBuilder.String(),
		})
		if err != nil {
			result.Content <- &AskStreamResultContent{
				Round:   curIter,
				Content: err.Error(),
			}
			break
		}

		if len(dstTcs) == 0 {
			// no tool calls
			break
		}

		for _, tcr := range dstTcs {
			if err := a.contextMgr.AppendToolResult(inMsg, &tcr.tc, tcr.result); err != nil {
				result.Content <- &AskStreamResultContent{
					Round:   curIter,
					Content: err.Error(),
				}
				break mainLoop
			}
		}
	}

	close(result.ToolCall)
	close(result.Content)
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
