package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ryanreadbooks/tokkibot/pkg/trace"
)

var (
	logFile     *os.File
	logFileMu   sync.Mutex
	currentDate string
	logsDir     string
)

// Init initializes the slog logger to write to date-based log files.
// Log files are stored in {logsDirectory}/{YYYY-MM-DD}.log
func Init(logsDirectory string) error {
	logsDir = logsDirectory
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	if err := rotateLogFile(); err != nil {
		return fmt.Errorf("failed to initialize log file: %w", err)
	}

	return nil
}

// rotateLogFile checks if the date has changed and rotates the log file if necessary.
func rotateLogFile() error {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	today := time.Now().Format("2006-01-02")
	if currentDate == today && logFile != nil {
		return nil
	}

	// Close the previous log file if exists
	if logFile != nil {
		logFile.Close()
	}

	logPath := filepath.Join(logsDir, today+".log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %w", logPath, err)
	}

	logFile = f
	currentDate = today

	// Create a JSON handler with time formatting and short source paths
	jsonHandler := slog.NewJSONHandler(logFile, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				t := a.Value.Time()
				a.Value = slog.StringValue(t.Format("2006-01-02 15:04:05.000"))
			}
			if a.Key == slog.SourceKey {
				if src, ok := a.Value.Any().(*slog.Source); ok {
					// Keep only filename instead of full path
					a.Value = slog.AnyValue(&slog.Source{
						Function: src.Function,
						File:     filepath.Base(src.File),
						Line:     src.Line,
					})
				}
			}
			return a
		},
	})

	// Wrap with trace handler to extract trace info from context
	traceHandler := &TraceHandler{Handler: jsonHandler}

	slog.SetDefault(slog.New(traceHandler))

	return nil
}

// TraceHandler wraps a slog.Handler to add trace info from context
type TraceHandler struct {
	slog.Handler
}

// Handle adds trace info from context before delegating to the wrapped handler
func (h *TraceHandler) Handle(ctx context.Context, r slog.Record) error {
	if traceInfo := trace.FromContext(ctx); traceInfo != nil {
		if traceInfo.ReqID != "" {
			r.AddAttrs(slog.String(trace.LogKeyReqID, traceInfo.ReqID))
		}
		if traceInfo.Channel != "" {
			r.AddAttrs(slog.String(trace.LogKeyChannel, traceInfo.Channel))
		}
		if traceInfo.ChatID != "" {
			r.AddAttrs(slog.String(trace.LogKeyChatID, traceInfo.ChatID))
		}
		if traceInfo.MessageID != "" {
			r.AddAttrs(slog.String(trace.LogKeyMessageID, traceInfo.MessageID))
		}
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs returns a new handler with the given attributes
func (h *TraceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TraceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

// WithGroup returns a new handler with the given group name
func (h *TraceHandler) WithGroup(name string) slog.Handler {
	return &TraceHandler{Handler: h.Handler.WithGroup(name)}
}

// Close closes the log file. Should be called on application shutdown.
func Close() {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}
