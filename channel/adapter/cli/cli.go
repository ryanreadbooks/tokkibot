package cli

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ryanreadbooks/tokkibot/channel/adapter"
	"github.com/ryanreadbooks/tokkibot/channel/model"
)

var _ adapter.Adapter = (*CLIAdapter)(nil)

type CLIAdapter struct {
	chatID string

	input  chan *model.IncomingMessage
	output chan *model.OutgoingMessage
}

type CLIConfig struct {
	ChatID string
}

func NewAdapter(cfg CLIConfig) *CLIAdapter {
	chatID := cfg.ChatID
	if chatID == "" {
		chatID = uuid.New().String()
	}

	return &CLIAdapter{
		chatID: chatID,
		input:  make(chan *model.IncomingMessage, 1),
		output: make(chan *model.OutgoingMessage, 16),
	}
}

func (a *CLIAdapter) Type() model.Type {
	return model.CLI
}

func (a *CLIAdapter) ReceiveChan() <-chan *model.IncomingMessage {
	return a.input
}

func (a *CLIAdapter) SendChan() chan<- *model.OutgoingMessage {
	return a.output
}

func (a *CLIAdapter) ChatID() string {
	return a.chatID
}

func (a *CLIAdapter) Start(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

type SendMessageResult struct {
	Content  <-chan *model.StreamContent
	ToolCall <-chan *model.StreamTool
}

func (a *CLIAdapter) SendUserMessage(ctx context.Context, content string) *SendMessageResult {
	contentCh := make(chan *model.StreamContent, 16)
	toolCh := make(chan *model.StreamTool, 16)

	msg := &model.IncomingMessage{
		Channel:   model.CLI,
		ChatId:    a.chatID,
		Created:   time.Now().Unix(),
		Content:   content,
		SourceCtx: ctx,
		Stream:    true,
		Metadata: map[string]any{
			"message_id": uuid.New().String(),
		},
		OnContent: func(c *model.StreamContent) {
			select {
			case contentCh <- c:
			case <-ctx.Done():
			}
		},
		OnTool: func(t *model.StreamTool) {
			select {
			case toolCh <- t:
			case <-ctx.Done():
			}
		},
		OnDone: func() {
			close(contentCh)
			close(toolCh)
		},
	}

	select {
	case a.input <- msg:
	case <-ctx.Done():
		close(contentCh)
		close(toolCh)
	}

	return &SendMessageResult{
		Content:  contentCh,
		ToolCall: toolCh,
	}
}
