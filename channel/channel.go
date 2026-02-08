package channel

import (
	"context"

	"github.com/ryanreadbooks/tokkibot/channel/model"
)

type BaseChannel interface {
	Type() model.Type
}

type OneDirectionChannel[T any] interface {
	BaseChannel

	// Wait for message from the channel.
	Wait(ctx context.Context) <-chan T

	// Send message to channel.
	Send(ctx context.Context, msg T) error
}

type IncomingChannel interface {
	OneDirectionChannel[model.IncomingMessage]
}

type OutgoingChannel interface {
	OneDirectionChannel[model.OutgoingMessage]
}
