package safe

import (
	"log/slog"
	"runtime/debug"
)

func Go(f func()) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("[safe] go panic", "error", err, "stack", string(debug.Stack()))
			}
		}()

		f()
	}()
}
