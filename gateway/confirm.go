package gateway

import (
	"context"

	"github.com/ryanreadbooks/tokkibot/channel/model"
	"github.com/ryanreadbooks/tokkibot/component/tool"
)

// ConfirmHandler implements tool.ToolConfirmer using IncomingMessage callbacks
type ConfirmHandler struct {
	msg *model.IncomingMessage
}

// NewConfirmHandler creates a new ConfirmHandler
func NewConfirmHandler(msg *model.IncomingMessage) *ConfirmHandler {
	return &ConfirmHandler{msg: msg}
}

// RequestConfirm implements tool.ToolConfirmer
func (h *ConfirmHandler) RequestConfirm(ctx context.Context, req *tool.ConfirmRequest) (*tool.ConfirmResponse, error) {
	if h.msg.OnConfirmWaiting == nil {
		return &tool.ConfirmResponse{Confirmed: true}, nil
	}

	respCh := make(chan *model.ConfirmResponse, 1)

	// Convert tool.ConfirmRequest to model.ConfirmRequest
	modelReq := &model.ConfirmRequest{
		Channel:     model.Type(req.Channel),
		ChatId:      req.ChatId,
		ToolName:    req.ToolName,
		Level:       int(req.Level),
		Title:       req.Title,
		Description: req.Description,
		Command:     req.Command,
		Metadata:    h.msg.Metadata,
	}

	// Emit confirmation request through message callback
	h.msg.EmitConfirm(&model.ConfirmEvent{
		Request: modelReq,
		RespCh:  respCh,
	})

	// Wait for confirm response or context cancellation
	select {
	case resp := <-respCh:
		return &tool.ConfirmResponse{
			Confirmed: resp.Confirmed,
			Reason:    resp.Reason,
		}, nil
	case <-ctx.Done():
		return &tool.ConfirmResponse{Confirmed: false, Reason: "cancelled"}, ctx.Err()
	}
}
