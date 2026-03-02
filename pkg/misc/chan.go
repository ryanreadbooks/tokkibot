package misc

import "github.com/ryanreadbooks/tokkibot/pkg/safe"

func DiscardChan[T any](ch <-chan T) {
	safe.Go(func() {
		for range ch {
		}
	})
}
