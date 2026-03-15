package agent

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/ryanreadbooks/tokkibot/component/tool"
	schema "github.com/ryanreadbooks/tokkibot/llm/schema"
)

const (
	xToolMetaMessageChannelKey = "x-tool-meta-message-channel"
)

func getXToolMetaMessageChannel(meta tool.InvokeMeta) *AskTemporaryMessageChannel {
	if val, ok := meta.Extras[xToolMetaMessageChannelKey]; ok {
		if out, ok := val.(*AskTemporaryMessageChannel); ok {
			return out
		}
	}

	return nil
}

// formatCancelledError formats the context cancellation error message
func formatCancelledError(ctx context.Context) string {
	return fmt.Sprintf("(operation cancelled by user: %s)", ctx.Err().Error())
}

// initMessageContext initializes session logs and appends the user message
func (a *Agent) initMessageContext(_ context.Context, userMsg *UserMessage) error {
	a.contextManager.InitFromSessionLogs(userMsg.Channel, userMsg.ChatId)
	_, err := a.contextManager.AppendUserMessage(userMsg)
	return err
}

func (a *Agent) handleIncomingMessage(
	ctx context.Context,
	userMsg *UserMessage,
	opt *askOptionImpl,
) (result string) {
	defer func() {
		if err := recover(); err != nil {
			slog.ErrorContext(ctx, "[agent] message handling panic",
				slog.Any("error", err),
				slog.String("stack", string(debug.Stack())))

			result = "I encountered an error while processing your request. Please try again later."
		}
	}()
	slog.InfoContext(ctx, "[agent] handling incoming message", slog.Int("content_len", len(userMsg.Content)))

	if err := a.initMessageContext(ctx, userMsg); err != nil {
		slog.ErrorContext(ctx, "[agent] failed to init message context", slog.Any("error", err))
		return err.Error()
	}

	toolMeta := tool.InvokeMeta{
		Channel: userMsg.Channel,
		ChatId:  userMsg.ChatId,
		Extras: map[string]any{
			xToolMetaMessageChannelKey: opt.messageChannel,
		},
	}

	var lastResponse *schema.Response
	for curIter := 1; curIter <= a.cfg.MaxIteration; curIter++ {
		select {
		case <-ctx.Done():
			slog.WarnContext(ctx, "[agent] message handling cancelled", slog.Int("iteration", curIter))
			return formatCancelledError(ctx)
		default:
		}

		llmReq, err := a.buildLLMMessageRequest(ctx, userMsg)
		if err != nil {
			slog.ErrorContext(ctx, "[agent] failed to build llm request", slog.Int("iteration", curIter), slog.Any("error", err))
			return fmt.Sprintf("(failed to build llm message request: %s)", err.Error())
		}

		startTime := time.Now()
		llmResp, err := a.llm.ChatCompletion(ctx, llmReq)
		if err != nil {
			slog.ErrorContext(ctx, "[agent] LLM call failed",
				slog.Int("iteration", curIter),
				slog.Int64("duration_ms", time.Since(startTime).Milliseconds()),
				slog.Any("error", err),
				slog.String("model", llmReq.Model),
				slog.Int("messages_count", len(llmReq.Messages)),
				slog.Int("tools_count", len(llmReq.Tools)),
			)
			return fmt.Sprintf("(failed to call llm: %s)", err.Error())
		}

		lastResponse = llmResp
		choice := llmResp.FirstChoice()
		if err := a.contextManager.AppendAssistantMessage(userMsg, &choice.Message); err != nil {
			slog.ErrorContext(ctx, "[agent] failed to append assistant message", slog.Any("error", err))
			return err.Error()
		}

		if choice.IsStopped() {
			slog.InfoContext(ctx, "[agent] conversation completed",
				slog.Int("iterations", curIter),
				slog.Int("response_len", len(choice.Message.Content)),
				slog.String("model", llmReq.Model),
				slog.Int("messages_count", len(llmReq.Messages)),
				slog.Int("tools_count", len(llmReq.Tools)),
			)
			return choice.Message.Content
		}

		if choice.HasToolCalls() {
			slog.DebugContext(ctx, "[agent] processing tool calls", slog.Int("count", len(choice.Message.ToolCalls)))
			for _, tc := range choice.Message.ToolCalls {
				if err := a.handleToolCall(ctx, toolMeta, userMsg, &tc); err != nil {
					slog.ErrorContext(ctx, "[agent] tool call failed", slog.String("tool", tc.Function.Name), slog.Any("error", err))
					return err.Error()
				}
			}
		}

		// auto-push completed subagent results
		if subResults := a.subAgentToolDelegate.receiveSubAgentResults(); subResults != "" {
			a.contextManager.AppendContextUserMessage(&UserMessage{
				Channel: userMsg.Channel,
				ChatId:  userMsg.ChatId,
				Created: time.Now().Unix(),
				Content: subResults,
			})
		}
	}

	slog.WarnContext(ctx, "[agent] max iterations reached", slog.Int("max_iteration", a.cfg.MaxIteration))
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
func (a *Agent) handleStreamingToolCall(
	toolMeta tool.InvokeMeta, dstTcs *[]*toolCallAndResult,
) schema.StreamToolCallHandler {
	dstTcsMu := sync.Mutex{}
	return func(ctx context.Context, tc schema.StreamChoiceDeltaToolCall) {
		// invoke tool
		result := a.getToolAndInvoke(ctx, toolMeta, &schema.CompletionToolCall{
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
	userMsg *UserMessage,
	emitter StreamEmitter,
	opt *askOptionImpl,
) {
	defer func() {
		if err := recover(); err != nil {
			slog.ErrorContext(ctx, "[agent] message handling panic",
				slog.Any("error", err),
				slog.String("stack", string(debug.Stack())))
			emitter.EmitContent(&EmittedContent{
				Round:   -1,
				Content: "I encountered an error while processing your request. Please try again later.",
			})
		}
	}()
	defer emitter.EmitDone()

	if err := a.initMessageContext(ctx, userMsg); err != nil {
		emitter.EmitContent(&EmittedContent{Round: -1, Content: err.Error()})
		return
	}

	toolMeta := tool.InvokeMeta{
		Channel: userMsg.Channel,
		ChatId:  userMsg.ChatId,
		Extras: map[string]any{
			xToolMetaMessageChannelKey: opt.messageChannel,
		},
	}

mainLoop:
	for curIter := 1; curIter <= a.cfg.MaxIteration; curIter++ {
		select {
		case <-ctx.Done():
			emitter.EmitContent(&EmittedContent{Round: curIter, Content: formatCancelledError(ctx)})
			break mainLoop
		default:
		}

		var (
			wg                      sync.WaitGroup
			contentBuilder          strings.Builder
			reasoningContentBuilder strings.Builder
			reasoningSignature      string
			dstTcs                  = make([]*toolCallAndResult, 0)
		)

		llmReq, err := a.buildLLMMessageRequest(ctx, userMsg)
		if err != nil {
			emitter.EmitContent(&EmittedContent{Round: curIter, Content: err.Error()})
			break
		}
		// call llm the stream way
		llmRespCh := a.llm.ChatCompletionStream(ctx, llmReq)
		streamPacked := schema.StreamResponseHandler(
			ctx,
			llmRespCh,
			a.handleStreamingToolCall(toolMeta, &dstTcs),
		)

		// accumluate content and reasoning content
		thinkingEnabled := llmReq.ThinkingEnabled()
		wg.Go(func() {
			for content := range streamPacked.Content {
				emitter.EmitContent(&EmittedContent{
					Round:            curIter,
					Content:          content.Content,
					ReasoningContent: content.ReasoningContent,
					Metadata:         EmittedReasoningContentMetadata{ThinkingEnabled: thinkingEnabled},
				})
				contentBuilder.WriteString(content.Content)
				reasoningContentBuilder.WriteString(content.ReasoningContent)
				if content.ReasoningSignature != "" {
					reasoningSignature = content.ReasoningSignature
				}
			}
		})

		// accumulate tool call
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

		err = a.contextManager.AppendAssistantMessage(userMsg, &schema.CompletionMessage{
			Content:   contentBuilder.String(),
			ToolCalls: assistantTcs,
			ReasoningContent: &schema.ReasoningContent{
				Content:   reasoningContentBuilder.String(),
				Signature: reasoningSignature,
			},
		})
		if err != nil {
			emitter.EmitContent(&EmittedContent{Round: curIter, Content: err.Error()})
			break
		}

		if len(dstTcs) == 0 {
			break
		}

		// IMPORTANT: must save all tool results to match assistant's tool_calls
		// even if ctx is cancelled, we need to complete the tool call sequence
		for _, tcr := range dstTcs {
			if err := a.contextManager.AppendToolResult(userMsg, &tcr.tc, tcr.result); err != nil {
				emitter.EmitContent(&EmittedContent{Round: curIter, Content: err.Error()})
				break mainLoop
			}
		}

		// auto-push completed subagent results
		if subResults := a.subAgentToolDelegate.receiveSubAgentResults(); subResults != "" {
			a.contextManager.AppendContextUserMessage(&UserMessage{
				Channel: userMsg.Channel,
				ChatId:  userMsg.ChatId,
				Created: time.Now().Unix(),
				Content: subResults,
			})
		}
	}
}

func (a *Agent) handleToolCall(
	ctx context.Context,
	toolMeta tool.InvokeMeta,
	inMsg *UserMessage,
	tc *schema.CompletionToolCall,
) error {
	toolResult := a.getToolAndInvoke(ctx, toolMeta, tc)
	// feedback tool calling result to llm
	return a.contextManager.AppendToolResult(inMsg, tc, toolResult)
}

func (a *Agent) getToolAndInvoke(
	ctx context.Context,
	toolMeta tool.InvokeMeta,
	tc *schema.CompletionToolCall,
) string {
	select {
	case <-ctx.Done():
		slog.DebugContext(ctx, "[agent] tool invoke cancelled", slog.String("tool", tc.Function.Name))
		return formatCancelledError(ctx)
	default:
	}

	slog.DebugContext(ctx, "[agent] invoking tool",
		slog.String("tool", tc.Function.Name),
		slog.Int("args_len", len(tc.Function.Arguments)),
		slog.String("arguments", tc.Function.Arguments),
	)
	startTime := time.Now()

	// try builtin tools first
	a.toolsMu.RLock()
	builtinTool, ok := a.tools[tc.Function.Name]
	a.toolsMu.RUnlock()

	if ok {
		result, err := builtinTool.Invoke(ctx, toolMeta, tc.Function.Arguments)
		duration := time.Since(startTime).Milliseconds()
		if err != nil {
			slog.WarnContext(ctx, "[agent] builtin tool error",
				slog.String("tool", tc.Function.Name),
				slog.Int64("duration_ms", duration),
				slog.Any("error", err),
				slog.String("arguments", tc.Function.Arguments),
			)
			return err.Error()
		}
		slog.InfoContext(ctx, "[agent] builtin tool completed",
			slog.String("tool", tc.Function.Name),
			slog.Int64("duration_ms", duration),
			slog.Int("result_len", len(result)),
			slog.String("arguments", tc.Function.Arguments),
		)
		return result
	}

	// fallback to mcp tools
	if a.mcpLoaded.Load() {
		if mcpTool, found := a.mcpManager.GetTool(tc.Function.Name); found {
			result, err := mcpTool.Invoke(ctx, toolMeta, tc.Function.Arguments)
			duration := time.Since(startTime).Milliseconds()
			if err != nil {
				slog.WarnContext(ctx, "[agent] mcp tool error",
					slog.String("tool", tc.Function.Name),
					slog.Int64("duration_ms", duration),
					slog.Any("error", err),
					slog.String("arguments", tc.Function.Arguments),
				)
				return err.Error()
			}
			slog.InfoContext(ctx, "[agent] mcp tool completed",
				slog.String("tool", tc.Function.Name),
				slog.Int64("duration_ms", duration),
				slog.Int("result_len", len(result)),
				slog.String("arguments", tc.Function.Arguments),
			)
			return result
		}
	}

	slog.WarnContext(ctx, "[agent] tool not found", slog.String("tool", tc.Function.Name))
	return fmt.Sprintf("(tool %s not found)", tc.Function.Name)
}
