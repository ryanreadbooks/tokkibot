package safe

import "log/slog"

func Go(f func()) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("[safe] go panic", "error", err)
			}
		}()

		f()
	}()
}
