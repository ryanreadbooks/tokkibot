package context

import (
	"context"
	"fmt"

	"github.com/ryanreadbooks/tokkibot/agent/context/session"
	"github.com/ryanreadbooks/tokkibot/component/skill"
	"github.com/ryanreadbooks/tokkibot/llm/schema"
	"github.com/ryanreadbooks/tokkibot/llm/schema/param"
)

// ContextManager is the runtime context abstraction used by agent.
//
// Persistent implementation stores data on disk.
// Volatile implementation keeps everything in memory only.
type ContextManager interface {
	InitFromSessionLogs(channel, chatId string)
	InitSession(channel, chatId string) error

	AppendContextUserMessage(inMsg *UserInput) ([]param.Message, error)
	AppendUserMessage(inMsg *UserInput) ([]param.Message, error)
	AppendToolResult(inMsg *UserInput, toolCall *schema.CompletionToolCall, result string) error
	AppendAssistantMessage(inMsg *UserInput, msg *schema.CompletionMessage) error

	GetMessageContext(channel, chatId string) ([]param.Message, error)
	GetSystemPrompt() string
	GetMessageHistory(channel, chatId string) ([]session.LogItem, error)

	ClearSession(channel, chatId string) error
	CompressToolCalls(channel, chatId string, count int) (int, error)
	SummarizeHistory(
		ctx context.Context,
		channel, chatId string,
		llmFunc func(context.Context, []param.Message) (string, error),
	) error
}

func NewContextManager(
	ctx context.Context,
	c ContextManagerConfig,
	skillLoader *skill.Loader,
) (ContextManager, error) {
	if c.Volatile {
		mgr, err := NewVolatileContextManager(ctx, c, skillLoader)
		if err != nil {
			return nil, fmt.Errorf("failed to create volatile context manager: %w", err)
		}
		return mgr, nil
	}

	mgr, err := NewPersistentContextManager(ctx, c, skillLoader)
	if err != nil {
		return nil, fmt.Errorf("failed to create persistent context manager: %w", err)
	}
	return mgr, nil
}

var (
	_ ContextManager = (*PersistentContextManager)(nil)
	_ ContextManager = (*VolatileContextManager)(nil)
)
