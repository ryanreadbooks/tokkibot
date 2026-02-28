package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	imv1 "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/ryanreadbooks/tokkibot/channel/adapter/lark/card"
	"github.com/ryanreadbooks/tokkibot/pkg/xstring"
)

func (a *LarkAdapter) sendMessage(ctx context.Context, userOpenId, msgType string, content string) {
	reqBuilder := imv1.NewCreateMessageReqBuilder()
	reqBuilder.ReceiveIdType(imv1.ReceiveIdTypeOpenId).
		Body(
			imv1.NewCreateMessageReqBodyBuilder().
				ReceiveId(userOpenId).
				MsgType(msgType).
				Content(content).
				Uuid(uuid.NewString()).
				Build(),
		)

	req := reqBuilder.Build()

	resp, err := a.cli.Im.Message.Create(ctx, req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to send error log to lark", "error", err)
		return
	}

	if !resp.Success() {
		slog.ErrorContext(ctx, "send lark message failed", "error", resp.ErrorResp(), "request_id", resp.RequestId())
		return
	}
}

// Send error log to lark to display to user.
func (a *LarkAdapter) sendErrorLog(ctx context.Context, userOpenId string, err error) {
	if err == nil {
		return
	}

	a.sendMessage(ctx, userOpenId, imv1.MsgTypeText, fmt.Sprintf(`{"text":"%s"}`, err.Error()))
}

// Send card message to lark.
// Content will be parsed and rendered as lark card elements.
// Content is considered as markdown format
func (a *LarkAdapter) sendInteractiveMessage(ctx context.Context, userOpenId string, content string) {
	cd := card.NewCardV2Builder().
		AppendBodyElement(card.NewCardV2BodyMarkdownElement(content)).
		Build()

	cdJson, err := json.Marshal(cd)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal card", "error", err)
		return
	}

	a.sendMessage(ctx, userOpenId, imv1.MsgTypeInteractive, xstring.FromBytes(cdJson))
}

func (a *LarkAdapter) replyMessage(ctx context.Context, messageId string, msgType string, content string) {
	body := imv1.NewReplyMessageReqBodyBuilder().
		MsgType(msgType).
		Content(content).
		Uuid(uuid.NewString()).
		Build()
	req := imv1.NewReplyMessageReqBuilder().
		MessageId(messageId).
		Body(body).
		Build()
	resp, err := a.cli.Im.Message.Reply(ctx, req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to reply message", "error", err)
		return
	}

	if !resp.Success() {
		slog.ErrorContext(ctx, "failed to reply message", "error", resp.ErrorResp(), "request_id", resp.RequestId())
		return
	}
}

func (a *LarkAdapter) replyInteractiveMessage(ctx context.Context, messageId string, content string) {
	cd := card.NewCardV2Builder().
		AppendBodyElement(card.NewCardV2BodyMarkdownElement(content)).
		Build()

	cdJson, err := json.Marshal(cd)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal card", "error", err)
		return
	}

	a.replyMessage(ctx, messageId, imv1.MsgTypeInteractive, xstring.FromBytes(cdJson))
}

// return reaction id
func (a *LarkAdapter) sendMessageReaction(ctx context.Context, messageId string, emojiType string) string {
	emoji := imv1.NewEmojiBuilder().EmojiType(emojiType).Build()
	body := imv1.NewCreateMessageReactionReqBodyBuilder().
		ReactionType(emoji).
		Build()
	req := imv1.NewCreateMessageReactionReqBuilder().
		MessageId(messageId).
		Body(body).
		Build()

	resp, err := a.cli.Im.MessageReaction.Create(ctx, req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to send message reaction", "error", err)
		return ""
	}

	if !resp.Success() {
		slog.ErrorContext(ctx, "failed to send message reaction", "error", resp.ErrorResp(), "request_id", resp.RequestId())
		return ""
	}

	if resp.Data != nil && resp.Data.ReactionId != nil {
		return *resp.Data.ReactionId
	}

	return ""
}

func (a *LarkAdapter) deleteMessageReaction(ctx context.Context, messageId, reactionId string) {
	req := imv1.NewDeleteMessageReactionReqBuilder().
		MessageId(messageId).
		ReactionId(reactionId).
		Build()

	resp, err := a.cli.Im.MessageReaction.Delete(ctx, req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete message reaction", "error", err)
		return
	}

	if !resp.Success() {
		slog.ErrorContext(ctx, "failed to delete message reaction", "error", resp.ErrorResp(), "request_id", resp.RequestId())
		return
	}
}
