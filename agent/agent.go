package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/ryanreadbooks/tokkibot/agent/tools"
	"github.com/ryanreadbooks/tokkibot/channel"
	channelmodel "github.com/ryanreadbooks/tokkibot/channel/model"
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

	// cwd
	WorkingDir string

	// The model to use.
	Model string

	ResumeSessionId string
}

type Agent struct {
	c AgentConfig
	// The LLM service to use.
	llm llm.LLM

	toolsMu sync.RWMutex
	tools   map[string]tool.Invoker

	// The context manager for the agent.
	contextMgr *ContextManager

	// incoming/outgoing channel bus
	bus *channel.Bus
}

func NewAgent(
	llm llm.LLM,
	bus *channel.Bus,
	c AgentConfig,
) *Agent {
	contextMgr, err := NewContextManage(c.RootCtx, ContextManagerConfig{
		workspace: config.GetConfigDir(),
	})
	if err != nil {
		slog.Error("[agent] failed to create context manager, now exit", "error", err)
		os.Exit(1)
	}

	agent := &Agent{
		c:          c,
		tools:      make(map[string]tool.Invoker),
		contextMgr: contextMgr,

		llm: llm,
		bus: bus,
	}

	agent.registerTools()

	return agent
}

// register tools
func (a *Agent) registerTools() {
	allowDirs := []string{a.c.WorkingDir, config.GetConfigDir()}
	a.RegisterTool(tools.ReadFile(allowDirs))
	a.RegisterTool(tools.WriteFile(allowDirs))
	a.RegisterTool(tools.ListDir(allowDirs))
	a.RegisterTool(tools.EditFile(allowDirs))
	a.RegisterTool(tools.Shell())
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

func (a *Agent) Run(ctx context.Context) error {
	if len(a.bus.IncomingChannels()) == 0 {
		slog.Warn("[agent] no input channels registered, will not start listening")
		return nil
	}

	for _, channel := range a.bus.IncomingChannels() {
		// TODO init all channel listening
		go func() {
			defer func() {
			}()

			a.loop(ctx, channel)
		}()
	}

	return nil
}

func (a *Agent) loop(ctx context.Context, channel channel.IncomingChannel) error {
	for {
		select {
		case <-ctx.Done():
			switch err := ctx.Err(); err {
			case context.Canceled:
				slog.Info("[agent] exited")
				return nil
			default:
				slog.Warn("[agent] exited", "error", err)
				return err
			}
		case inMsg, ok := <-channel.Wait(ctx):
			if !ok {
				// channel is closed
				slog.Info("[agent] channel closed", "channel", channel.Type())
			} else {
				answer := a.handleIncomingMessage(ctx, &inMsg)
				a.sendOutgoingMessage(ctx, inMsg.Channel, inMsg.ChatId, answer)
			}
		}
	}
}

func (a *Agent) handleIncomingMessage(ctx context.Context, inMsg *channelmodel.IncomingMessage) string {
	curIter := 0
	finalResult := ""

	a.contextMgr.AppendUserMessage(inMsg)
	for curIter <= maxIterAllowed {
		curIter++

		// build llm message request
		llmReq := a.buildLLMMessageRequest(inMsg)

		// call llm
		llmResp, err := a.llm.ChatCompletion(ctx, llmReq)
		if err != nil {
			slog.Warn("[agent] failed to call llm", "error", err, "channel", inMsg.Channel)
			continue
		}

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

	return finalResult
}

func (a *Agent) sendOutgoingMessage(ctx context.Context,
	chanType channelmodel.Type, chatId string, content string,
) {
	if outCh := a.bus.GetOutgoingChannel(chanType); outCh != nil {
		outCh.Send(ctx, channelmodel.OutgoingMessage{
			Channel: chanType,
			ChatId:  chatId,
			Content: content,
			Created: time.Now().Unix(),
		})
	}
}

func (a *Agent) handleToolCall(
	ctx context.Context,
	inMsg *channelmodel.IncomingMessage,
	tc *llmmodel.CompletionToolCall,
) {
	a.toolsMu.RLock()
	tool, ok := a.tools[tc.Function.Name]
	if !ok {
		a.toolsMu.RUnlock()
		return
	}

	// execute tool
	toolResult, err := tool.Invoke(ctx, tc.Function.Arguments)
	if err != nil {
		toolResult = fmt.Sprintf("Failed to invoke tool: %s", err.Error())
	}

	// feedback tool calling result to llm
	a.contextMgr.AppendToolResult(inMsg, tc, toolResult)
	a.toolsMu.RUnlock()
}

func (a *Agent) buildLLMMessageRequest(inMsg *channelmodel.IncomingMessage) *llm.Request {
	a.contextMgr.InitHistoryMessages(inMsg.Channel, inMsg.ChatId)
	r := &llm.Request{
		Model:    a.c.Model,
		Messages: a.contextMgr.GetMessageList(),
		Tools:    a.buildLLMToolParams(),
	}

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

func (a *Agent) RetrieveSession(channel channelmodel.Type, chatId string) ([]SessionMessage, error) {
	history, err := a.contextMgr.sessionMgr.GetSessionHistory(channel, chatId)
	if err != nil {
		return nil, err
	}

	return history, nil
}
