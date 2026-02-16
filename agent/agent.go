package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ryanreadbooks/tokkibot/agent/tools"
	"github.com/ryanreadbooks/tokkibot/channel"
	chmodel "github.com/ryanreadbooks/tokkibot/channel/model"
	"github.com/ryanreadbooks/tokkibot/component/skill"
	"github.com/ryanreadbooks/tokkibot/component/tool"
	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/llm"
	llmmodel "github.com/ryanreadbooks/tokkibot/llm/model"
)

const (
	maxIterAllowed = 30
)

type AgentConfig struct {
	RootCtx context.Context

	// The model to use.
	Model string

	ResumeSessionId string
}

type ThinkingState string

const (
	ThinkingStateThinking ThinkingState = "thinking"
	ThinkingStateDone     ThinkingState = "done"
)

type Agent struct {
	c AgentConfig
	// The LLM service to use.
	llm llm.LLM

	toolsMu sync.RWMutex
	tools   map[string]tool.Invoker

	// The context manager for the agent.
	contextMgr *ContextManager

	// skill loader
	skillLoader *skill.Loader

	// incoming/outgoing channel bus
	bus *channel.Bus
}

func NewAgent(
	llm llm.LLM,
	c AgentConfig,
) *Agent {
	sessionManager := NewSessionManager(c.RootCtx, SessionManagerConfig{
		workspace:    config.GetWorkspaceDir(),
		saveInterval: 10 * time.Second,
	})
	memoryManager := NewMemoryManager(MemoryManagerConfig{
		workspace: config.GetWorkspaceDir(),
	})

	skillLoader := skill.NewLoader()
	if err := skillLoader.Init(); err != nil {
		slog.Error("[agent] failed to init skill loader, now exit", "error", err)
		os.Exit(1)
	}

	contextMgr, err := NewContextManage(c.RootCtx,
		ContextManagerConfig{
			workspace: config.GetWorkspaceDir(),
		},
		sessionManager,
		memoryManager,
		skillLoader,
	)
	if err != nil {
		slog.Error("[agent] failed to create context manager, now exit", "error", err)
		os.Exit(1)
	}

	agent := &Agent{
		c:           c,
		tools:       make(map[string]tool.Invoker),
		contextMgr:  contextMgr,
		skillLoader: skillLoader,

		llm: llm,
	}

	agent.registerTools()

	return agent
}

// register tools
func (a *Agent) registerTools() {
	allowDirs := []string{config.GetProjectDir(), config.GetWorkspaceDir()}
	a.RegisterTool(tools.ReadFile(allowDirs))
	a.RegisterTool(tools.WriteFile(allowDirs))
	a.RegisterTool(tools.ListDir(allowDirs))
	a.RegisterTool(tools.EditFile(allowDirs))
	a.RegisterTool(tools.Shell())
	a.RegisterTool(tools.UseSkill(a.skillLoader))
}

func (a *Agent) RegisterTool(tool tool.Invoker) {
	if tool == nil {
		return
	}

	a.toolsMu.Lock()
	defer a.toolsMu.Unlock()

	if _, ok := a.tools[tool.Info().Name]; ok {
		slog.Warn("[agent] tool already registered", "tool_name", tool.Info().Name)
	} else {
		a.tools[tool.Info().Name] = tool
	}
}

func (a *Agent) Ask(ctx context.Context, msg *chmodel.IncomingMessage) string {
	return a.handleIncomingMessage(ctx, msg)
}

type AskStreamResultToolCall struct {
	Name      string
	Arguments string
}

type AskStreamResultContent struct {
	Content          string
	ReasoningContent string
}

type AskStreamResult struct {
	Content  chan *AskStreamResultContent
	ToolCall chan *AskStreamResultToolCall
}

func (a *Agent) AskStream(ctx context.Context, msg *chmodel.IncomingMessage) *AskStreamResult {
	res := AskStreamResult{
		Content:  make(chan *AskStreamResultContent),
		ToolCall: make(chan *AskStreamResultToolCall),
	}
	go a.handleIncomingMessageStream(ctx, msg, &res)

	return &res
}

func (a *Agent) handleIncomingMessage(ctx context.Context, inMsg *chmodel.IncomingMessage) string {
	curIter := 0
	finalResult := ""
	var lastResponse *llm.Response

	a.contextMgr.AppendUserMessage(inMsg)
	for curIter <= maxIterAllowed {
		curIter++

		// build llm message request
		llmReq := a.buildLLMMessageRequest(inMsg)

		// call llm
		llmResp, err := a.llm.ChatCompletion(ctx, llmReq)
		if err != nil {
			return fmt.Sprintf("(failed to call llm: %s)", err.Error())
		}
		lastResponse = llmResp

		choice := llmResp.FirstChoice()
		// append assitant messages
		a.contextMgr.AppendAssistantMessage(inMsg, &choice.Message)

		if choice.IsStopped() {
			// finished
			finalResult = choice.Message.Content
			break
		}

		if choice.HasToolCalls() {
			// TODO maybe in the future we need to handle tool calls concurrently
			tcs := choice.Message.ToolCalls
			for _, tc := range tcs {
				a.handleToolCall(ctx, inMsg, &tc)
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
	tc     llmmodel.CompletionToolCall
	result string
}

// handle streaming tool call
//
// This method will be called when one tool call response is completed.
// Tool call will be invoked from another goroutine.
func (a *Agent) handleStreamingToolCall(dstTcs *[]*toolCallAndResult) llm.StreamToolCallHandler {
	dstTcsMu := sync.Mutex{}
	return func(ctx context.Context, tc llmmodel.StreamChoiceDeltaToolCall) {
		// invoke tool
		result := a.getToolAndInvoke(ctx, &llmmodel.CompletionToolCall{
			Id:       tc.Id,
			Type:     tc.Type,
			Function: tc.Function,
		})

		dstTcsMu.Lock()
		defer dstTcsMu.Unlock()
		*dstTcs = append(*dstTcs, &toolCallAndResult{
			tc: llmmodel.CompletionToolCall{
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
	inMsg *chmodel.IncomingMessage,
	result *AskStreamResult,
) {
	curIter := 0
	a.contextMgr.AppendUserMessage(inMsg)

	for curIter <= maxIterAllowed {
		var (
			wg                      sync.WaitGroup
			contentBuilder          strings.Builder
			reasoningContentBuilder strings.Builder
			// we also need to accumulate tool calls from assistant to feed back to llm
			dstTcs = make([]*toolCallAndResult, 0)
		)

		curIter++
		llmReq := a.buildLLMMessageRequest(inMsg)
		// call llm the stream way
		llmRespCh := a.llm.ChatCompletionStream(ctx, llmReq)
		streamPacked := llm.StreamResponseHandler(ctx, llmRespCh, a.handleStreamingToolCall(&dstTcs))

		wg.Go(func() {
			for content := range streamPacked.Content {
				result.Content <- &AskStreamResultContent{
					Content:          content.Content,
					ReasoningContent: content.ReasoningContent,
				}
				contentBuilder.WriteString(content.Content)
				reasoningContentBuilder.WriteString(content.ReasoningContent)
			}
		})

		wg.Go(func() {
			for toolCall := range streamPacked.ToolCall {
				select {
				case result.ToolCall <- &AskStreamResultToolCall{
					Name:      toolCall.Name,
					Arguments: toolCall.ArgumentFragment,
				}:
				default:
				}
			}
		})

		wg.Wait()

		assistantTcs := make([]llmmodel.CompletionToolCall, 0, len(dstTcs))
		for _, tcr := range dstTcs {
			assistantTcs = append(assistantTcs, tcr.tc)
		}

		// accumulate assistant message for this iteration
		a.contextMgr.AppendAssistantMessage(inMsg, &llmmodel.CompletionMessage{
			Content:          contentBuilder.String(),
			ToolCalls:        assistantTcs,
			ReasoningContent: reasoningContentBuilder.String(),
		})

		if len(dstTcs) == 0 {
			// no tool calls, we are done
			break
		}

		// add tool call results
		for _, tcr := range dstTcs {
			a.contextMgr.AppendToolResult(inMsg, &tcr.tc, tcr.result)
		}
	}

	// close all
	close(result.ToolCall)
	close(result.Content)
}

func (a *Agent) handleToolCall(
	ctx context.Context,
	inMsg *chmodel.IncomingMessage,
	tc *llmmodel.CompletionToolCall,
) {
	toolResult := a.getToolAndInvoke(ctx, tc)
	// feedback tool calling result to llm
	a.contextMgr.AppendToolResult(inMsg, tc, toolResult)
}

func (a *Agent) getToolAndInvoke(ctx context.Context, tc *llmmodel.CompletionToolCall) string {
	a.toolsMu.RLock()
	tool, ok := a.tools[tc.Function.Name]
	if !ok {
		a.toolsMu.RUnlock()
		return fmt.Sprintf("(tool %s not found)", tc.Function.Name)
	}

	// execute tool
	toolResult, err := tool.Invoke(ctx, tc.Function.Arguments)
	if err != nil {
		toolResult = err.Error()
	}

	a.toolsMu.RUnlock()
	return toolResult
}

func (a *Agent) buildLLMMessageRequest(inMsg *chmodel.IncomingMessage) *llm.Request {
	a.contextMgr.InitHistoryMessages(inMsg.Channel, inMsg.ChatId)
	r := llm.NewRequest(a.c.Model, a.contextMgr.GetMessageList())
	r.Temperature = 1
	r.MaxTokens = 16384
	r.Thinking = llmmodel.EnableThinking()
	r.Tools = a.buildLLMToolParams()

	return r
}

func (a *Agent) buildLLMToolParams() []llmmodel.ToolParam {
	params := make([]llmmodel.ToolParam, 0, len(a.tools))
	for _, tool := range a.tools {
		params = append(params, llmmodel.NewToolParamWithSchemaParam(
			tool.Info().Name, tool.Info().Description, *tool.Info().Schema,
		))
	}

	return params
}

func (a *Agent) RetrieveSession(channel chmodel.Type, chatId string) ([]SessionMessage, error) {
	history, err := a.contextMgr.sessionMgr.GetSessionHistory(channel, chatId)
	if err != nil {
		return nil, err
	}

	return history, nil
}

func (a *Agent) AvailableSkills() []*skill.Skill {
	return a.skillLoader.Skills()
}

func (a *Agent) GetSystemPrompt() string {
	return a.contextMgr.systemPrompts
}
