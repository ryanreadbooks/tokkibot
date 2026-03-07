package tool

import "context"

// ConfirmLevel defines the severity level of confirmation required
type ConfirmLevel int

const (
	ConfirmNone    ConfirmLevel = iota // No confirmation needed
	ConfirmNormal                      // Normal confirmation (e.g., rm commands)
	ConfirmDanger                      // Dangerous operation (may need extra confirmation)
	ConfirmBlocked                     // Completely blocked (will be rejected)
)

// ConfirmRequest represents a confirmation request from a tool
type ConfirmRequest struct {
	Channel     string         `json:"channel"`
	ChatId      string         `json:"chat_id"`
	ToolName    string         `json:"tool_name"`
	Level       ConfirmLevel   `json:"level"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Command     string         `json:"command"`
}

// ConfirmResponse represents the user's response to a confirmation request
type ConfirmResponse struct {
	Confirmed bool   `json:"confirmed"`
	Reason    string `json:"reason,omitempty"`
}

// ToolConfirmer is the unified interface for requesting user confirmation
type ToolConfirmer interface {
	// RequestConfirm sends a confirmation request and blocks until user responds
	RequestConfirm(ctx context.Context, req *ConfirmRequest) (*ConfirmResponse, error)
}

type confirmContextKey struct{}

// WithConfirmer injects a ToolConfirmer into context
func WithConfirmer(ctx context.Context, c ToolConfirmer) context.Context {
	return context.WithValue(ctx, confirmContextKey{}, c)
}

// GetConfirmer retrieves the ToolConfirmer from context
func GetConfirmer(ctx context.Context) (ToolConfirmer, bool) {
	c, ok := ctx.Value(confirmContextKey{}).(ToolConfirmer)
	return c, ok
}
