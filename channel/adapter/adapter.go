package adapter

import (
	"context"

	"github.com/ryanreadbooks/tokkibot/channel/model"
)

type Adapter interface {
	// Type returns the channel of the adapter.
	Type() model.Type

	// ReceiveChan returns a channel that receives incoming messages.
	ReceiveChan() <-chan *model.IncomingMessage

	// SendChan returns a channel that sends outgoing messages.
	SendChan() chan<- *model.OutgoingMessage

	// Start starts the adapter processing.
	// It should block until the context is done.
	Start(ctx context.Context) error
}
