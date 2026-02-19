package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	schema "github.com/ryanreadbooks/tokkibot/llm/schema"
)

func (a *Agent) handleIncomingMessage(ctx context.Context, inMsg *UserMessage) string {
	curIter := 0
	finalResult := ""
	var lastResponse *schema.Response
	a.contextMgr.InitFromSessionLogs(inMsg.Channel, inMsg.ChatId) // lazy init
	_, err := a.contextMgr.AppendUserMessage(inMsg)
	if err != nil {
		return err.Error()
	}

	for curIter <= maxIterAllowed {
		curIter++

		// build llm message request
		llmReq, err := a.buildLLMMessageRequest(ctx, inMsg)
		if err != nil {
			return fmt.Sprintf("(failed to build llm message request: %s)", err.Error())
		}

		// call llm
		llmResp, err := a.llm.ChatCompletion(ctx, llmReq)
		if err != nil {
			return fmt.Sprintf("(failed to call llm: %s)", err.Error())
		}
		lastResponse = llmResp

		choice := llmResp.FirstChoice()
		// append assitant messages
		if err := a.contextMgr.AppendAssistantMessage(inMsg, &choice.Message); err != nil {
			finalResult = err.Error()
			break
		}

		if choice.IsStopped() {
			// finished
			finalResult = choice.Message.Content
			break
		}

		if choice.HasToolCalls() {
			// TODO maybe in the future we need to handle tool calls concurrently
			tcs := choice.Message.ToolCalls
			var err error
			for _, tc := range tcs {
				err = a.handleToolCall(ctx, inMsg, &tc)
			}
			if err != nil {
				finalResult = err.Error()
				break
			}
		}
	}

	// if max iterations reached, and we still dont get a final result, use last response
	if finalResult == "" && lastResponse != nil {
		finalResult = fmt.Sprintf("(max iterations reached, last response: %s)", lastResponse.FirstChoice().Message.Content)
	}

	return finalResult
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
	curIter := 0
	a.contextMgr.InitFromSessionLogs(inMsg.Channel, inMsg.ChatId) // lazy init
	_, err := a.contextMgr.AppendUserMessage(inMsg)
	if err != nil {
		result.Content <- &AskStreamResultContent{
			Round:   -1,
			Content: err.Error(),
		}

		close(result.ToolCall)
		close(result.Content)
		return
	}

	for curIter <= maxIterAllowed {
		var (
			wg                      sync.WaitGroup
			contentBuilder          strings.Builder
			reasoningContentBuilder strings.Builder
			// we also need to accumulate tool calls from assistant to feed back to llm
			dstTcs = make([]*toolCallAndResult, 0)
		)

		curIter++
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
			for toolCall := range streamPacked.ToolCall {
				result.ToolCall <- &AskStreamResultToolCall{
					Round:     curIter,
					Name:      toolCall.Name,
					Arguments: toolCall.ArgumentFragment,
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
			// no tool calls, we are done
			break
		}

		// add tool call results
		for _, tcr := range dstTcs {
			err = a.contextMgr.AppendToolResult(inMsg, &tcr.tc, tcr.result)
		}
		if err != nil {
			result.Content <- &AskStreamResultContent{
				Round:   curIter,
				Content: err.Error(),
			}
			break
		}
	}

	// close all
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
