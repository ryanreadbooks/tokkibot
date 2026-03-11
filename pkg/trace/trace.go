package trace

import (
	"context"

	"github.com/ryanreadbooks/tokkibot/pkg/xstring"
)

// Log attribute keys for trace info
const (
	LogKeyReqID     = "req_id"
	LogKeyChannel   = "channel"
	LogKeyChatID    = "chat_id"
	LogKeyMessageID = "message_id"
)

type traceKey struct{}

// TraceInfo holds tracing information for a request
type TraceInfo struct {
	ReqID     string // unique request id
	Channel   string // message channel (cli, lark, etc.)
	ChatID    string // chat/conversation id
	MessageID string // message id (if available)
}

// WithTrace creates a new context with trace info
func WithTrace(ctx context.Context, info *TraceInfo) context.Context {
	return context.WithValue(ctx, traceKey{}, info)
}

// FromContext extracts trace info from context
func FromContext(ctx context.Context) *TraceInfo {
	if ctx == nil {
		return nil
	}
	if info, ok := ctx.Value(traceKey{}).(*TraceInfo); ok {
		return info
	}
	return nil
}

// NewTraceInfo creates a new TraceInfo with a generated request ID
func NewTraceInfo(channel, chatID, messageID string) *TraceInfo {
	return &TraceInfo{
		ReqID:     generateReqID(),
		Channel:   channel,
		ChatID:    chatID,
		MessageID: messageID,
	}
}

// generateReqID generates a unique request ID (16 chars, lowercase + digits)
func generateReqID() string {
	return xstring.RandomString(32, xstring.WithLowercaseOnly(true))
}
