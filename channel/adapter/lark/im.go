package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	imv1 "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/ryanreadbooks/tokkibot/channel/adapter/lark/card"
	"github.com/ryanreadbooks/tokkibot/channel/model"
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
		AppendBodyElement(card.NewBodyMarkdownElement(content)).
		Build()

	cdJson, err := json.Marshal(cd)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal card", "error", err)
		return
	}

	a.sendMessage(ctx, userOpenId, imv1.MsgTypeInteractive, xstring.FromBytes(cdJson))
}

func (a *LarkAdapter) replyMessage(ctx context.Context, messageId string, msgType string, content string) (string, error) {
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
		return "", err
	}

	if !resp.Success() {
		slog.ErrorContext(ctx, "failed to reply message", "error", resp.ErrorResp(), "request_id", resp.RequestId())
		return "", err
	}

	return *resp.Data.MessageId, nil
}

func (a *LarkAdapter) replyInteractiveMessage(ctx context.Context, messageId string, content string) (string, error) {
	cd := card.NewCardV2Builder().
		AppendBodyElement(card.NewBodyMarkdownElement(content)).
		Build()

	cdJson, err := json.Marshal(cd)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal card", "error", err)
		return "", err
	}

	return a.replyMessage(ctx, messageId, imv1.MsgTypeInteractive, xstring.FromBytes(cdJson))
}

func (a *LarkAdapter) replyInteractiveCardMessage(ctx context.Context, messageId, cardId string) (string, error) {
	cd := card.NewEntity(cardId)
	cdJson, err := json.Marshal(cd)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal card", "error", err)
		return "", err
	}

	return a.replyMessage(ctx, messageId, imv1.MsgTypeInteractive, xstring.FromBytes(cdJson))
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

func (a *LarkAdapter) recallMessage(ctx context.Context, messageId string) {
	req := imv1.NewDeleteMessageReqBuilder().
		MessageId(messageId).
		Build()
	resp, err := a.cli.Im.Message.Delete(ctx, req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to recall message", "error", err)
		return
	}

	if !resp.Success() {
		slog.ErrorContext(ctx, "failed to recall message", "error", resp.ErrorResp(), "request_id", resp.RequestId())
		return
	}
}

func (a *LarkAdapter) getMessage(ctx context.Context, messageId string) ([]*imv1.Message, error) {
	req := imv1.NewGetMessageReqBuilder().
		MessageId(messageId).
		UserIdType(imv1.UserIdTypeOpenId).
		Build()

	resp, err := a.cli.Im.V1.Message.Get(ctx, req)
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, fmt.Errorf("failed to get message: %s", resp.ErrorResp())
	}

	return resp.Data.Items, nil
}

// parsedMessage represents a parsed message with content and attachments
type parsedMessage struct {
	Content     string
	Attachments []*model.IncomingMessageAttachment
}

// getQuotedMessages retrieves the parsed content and attachments of quoted messages.
// Returns a slice since getMessage may return multiple items.
func (a *LarkAdapter) getQuotedMessages(ctx context.Context, messageId string) []*parsedMessage {
	items, err := a.getMessage(ctx, messageId)
	if err != nil || len(items) == 0 {
		slog.WarnContext(ctx, "failed to get quoted message", "message_id", messageId, "error", err)
		return nil
	}

	var results []*parsedMessage
	for _, msg := range items {
		if msg.Body == nil || msg.Body.Content == nil {
			continue
		}

		msgType := ""
		if msg.MsgType != nil {
			msgType = *msg.MsgType
		}

		msgId := messageId
		if msg.MessageId != nil {
			msgId = *msg.MessageId
		}

		if parsed := a.parseMessageByType(ctx, msgId, msgType, *msg.Body.Content); parsed != nil {
			results = append(results, parsed)
		}
	}

	return results
}

// parseMessageByType parses message content based on message type.
// This is the unified entry point for parsing any lark message.
func (a *LarkAdapter) parseMessageByType(ctx context.Context, messageId, msgType, rawContent string) *parsedMessage {
	var (
		content     string
		attachments []*model.IncomingMessageAttachment
		err         error
	)

	// See: https://open.feishu.cn/document/server-docs/im-v1/message/get
	// 卡片消息内容与在卡片搭建工具中获取的卡片 JSON 不一致，暂不支持返回原始卡片 JSON。
	// 暂不支持返回 JSON 2.0 卡片的具体内容。

	switch msgType {
	case imv1.MsgTypeText:
		content, err = a.handleTextMessage(rawContent)
	case imv1.MsgTypePost:
		content, attachments, err = a.handlePostMessage(ctx, rawContent, messageId)
	case imv1.MsgTypeImage:
		var imageKey string
		var imageData []byte
		imageKey, imageData, err = a.handleImageMessage(ctx, messageId, rawContent)
		if err == nil && len(imageData) > 0 {
			attachments = append(attachments, &model.IncomingMessageAttachment{
				Key:  wrapResourceKey(imageKey),
				Type: model.AttachmentImage,
				Data: imageData,
			})
		}
	default:
		return nil
	}

	if err != nil {
		slog.WarnContext(ctx, "failed to parse message", "message_id", messageId, "type", msgType, "error", err)
		return nil
	}

	return &parsedMessage{
		Content:     content,
		Attachments: attachments,
	}
}
