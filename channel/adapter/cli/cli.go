package cli

import (
	"context"
	"sync"
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

	mu        sync.Mutex
	callbacks map[string]*streamCallback
}

type streamCallback struct {
	contentCh chan *model.StreamContent
	toolCh    chan *model.StreamTool
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
		chatID:    chatID,
		input:     make(chan *model.IncomingMessage, 1),
		output:    make(chan *model.OutgoingMessage, 16),
		callbacks: make(map[string]*streamCallback),
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

	msgID := uuid.New().String()

	a.mu.Lock()
	a.callbacks[msgID] = &streamCallback{
		contentCh: contentCh,
		toolCh:    toolCh,
	}
	a.mu.Unlock()

	msg := &model.IncomingMessage{
		Channel:   model.CLI,
		ChatId:    a.chatID,
		Created:   time.Now().Unix(),
		Content:   content,
		SourceCtx: ctx,
		Stream:    true,
		Metadata: map[string]any{
			"message_id": msgID,
		},
	}
	msg.SetStreamContent(contentCh)
	msg.SetStreamTool(toolCh)

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
