package channel

import (
	"sync"

	"github.com/ryanreadbooks/tokkibot/channel/model"
)

type Bus struct {
	inMu  sync.RWMutex
	inChs map[model.Type]IncomingChannel

	outMu  sync.RWMutex
	outChs map[model.Type]OutgoingChannel
}

func NewBus() *Bus {
	b := &Bus{
		inChs:  make(map[model.Type]IncomingChannel),
		outChs: make(map[model.Type]OutgoingChannel),
	}

	return b
}

func (b *Bus) IncomingChannels() map[model.Type]IncomingChannel {
	b.inMu.RLock()
	defer b.inMu.RUnlock()
	return b.inChs
}

func (b *Bus) OutgoingChannels() map[model.Type]OutgoingChannel {
	b.outMu.RLock()
	defer b.outMu.RUnlock()
	return b.outChs
}

func (b *Bus) GetIncomingChannel(typ model.Type) IncomingChannel {
	b.inMu.RLock()
	defer b.inMu.RUnlock()
	return b.inChs[typ]
}

func (b *Bus) GetOutgoingChannel(typ model.Type) OutgoingChannel {
	b.outMu.RLock()
	defer b.outMu.RUnlock()
	return b.outChs[typ]
}

func (b *Bus) RegisterIncomingChannel(ch IncomingChannel) {
	b.inMu.Lock()
	defer b.inMu.Unlock()
	b.inChs[ch.Type()] = ch
}

func (b *Bus) RegisterOutgoingChannel(ch OutgoingChannel) {
	b.outMu.Lock()
	defer b.outMu.Unlock()
	b.outChs[ch.Type()] = ch
}
