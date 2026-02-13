package cli

import (
	"context"

	"github.com/ryanreadbooks/tokkibot/channel"
	"github.com/ryanreadbooks/tokkibot/channel/model"
)

// channel base on cli
type CliInputChannel struct {
	inputCh chan model.IncomingMessage
}

func (c *CliInputChannel) Type() model.Type {
	return model.ChannelCLI
}

var _ channel.IncomingChannel = (*CliInputChannel)(nil)

func (c *CliInputChannel) Send(ctx context.Context, msg model.IncomingMessage) error {
	c.inputCh <- msg
	return nil
}

func (c *CliInputChannel) Wait(ctx context.Context) <-chan model.IncomingMessage {
	return c.inputCh
}

type CliOutputChannel struct {
	outputCh chan model.OutgoingMessage
}

var _ channel.OutgoingChannel = (*CliOutputChannel)(nil)

func (c *CliOutputChannel) Type() model.Type {
	return model.ChannelCLI
}

func (c *CliOutputChannel) Send(ctx context.Context, msg model.OutgoingMessage) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.outputCh <- msg:
		return nil
	}
}

func (c *CliOutputChannel) Wait(ctx context.Context) <-chan model.OutgoingMessage {
	return c.outputCh
}

func NewCLIInputChannel() *CliInputChannel {
	return &CliInputChannel{
		inputCh: make(chan model.IncomingMessage, 1),
	}
}

func NewCLIOutputChannel() *CliOutputChannel {
	return &CliOutputChannel{
		outputCh: make(chan model.OutgoingMessage, 1),
	}
}
