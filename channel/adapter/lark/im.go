package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"

	"github.com/gabriel-vasile/mimetype"
	"github.com/ryanreadbooks/tokkibot/channel/adapter/lark/card"
	"github.com/ryanreadbooks/tokkibot/channel/model"
	"github.com/ryanreadbooks/tokkibot/pkg/xstring"

	"github.com/google/uuid"
	imv1 "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func (a *LarkAdapter) sendMessage(
	ctx context.Context,
	receiveIdType string,
	receiveId string,
	msgType string,
	content string,
) {
	reqBuilder := imv1.NewCreateMessageReqBuilder()
	reqBuilder.ReceiveIdType(receiveIdType).
		Body(
			imv1.NewCreateMessageReqBodyBuilder().
				ReceiveId(receiveId).
				MsgType(msgType).
				Content(content).
				Uuid(uuid.NewString()).
				Build(),
		)

	req := reqBuilder.Build()

	resp, err := a.cli.Im.Message.Create(ctx, req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to send to lark", "error", err)
		return
	}

	if !resp.Success() {
		slog.ErrorContext(ctx, "send lark message failed", "error", resp.ErrorResp(), "request_id", resp.RequestId())
		return
	}
}

func (a *LarkAdapter) sendError(ctx context.Context, userOpenId string, err error) {
	if err == nil {
		return
	}

	escapedError := html.EscapeString(err.Error())
	a.sendMessage(ctx, imv1.ReceiveIdTypeOpenId, userOpenId, imv1.MsgTypeText, fmt.Sprintf(`{"text":"%s"}`, escapedError))
}

func (a *LarkAdapter) sendCard(ctx context.Context, target messageTarget, content string) {
	cd := card.NewCardV2Builder().
		AppendBodyElement(card.NewBodyMarkdownElement(content)).
		Build()

	cdJson, err := json.Marshal(cd)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal card", "error", err)
		return
	}

	a.sendMessage(ctx, target.idType, target.id, imv1.MsgTypeInteractive, xstring.FromBytes(cdJson))
}

func (a *LarkAdapter) sendImage(ctx context.Context, target messageTarget, image []byte) {
	imgKey, err := a.uploadMessageResourceImage(ctx, image)
	if err != nil {
		slog.ErrorContext(ctx, "failed to upload image", "error", err)
		return
	}

	a.sendMessage(ctx, target.idType, target.id, imv1.MsgTypeImage, `{"image_key":"`+imgKey+`"}`)
}

func (a *LarkAdapter) sendAudio(ctx context.Context, target messageTarget, filename string, audio []byte, audioDuration int) {
	audioKey, err := a.uploadMessageResourceFile(ctx, uploadFileTypeOpus, filename, audio, &messageResourceFileExtra{
		audioDuration: audioDuration,
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to upload audio", "error", err)
		return
	}

	a.sendMessage(ctx, target.idType, target.id, imv1.MsgTypeAudio, `{"file_key":"`+audioKey+`"}`)
}

func (a *LarkAdapter) sendMedia(ctx context.Context, target messageTarget, filename string, media []byte) {
	mediaKey, err := a.uploadMessageResourceFile(ctx, uploadFileTypeMp4, filename, media, nil)
	if err != nil {
		slog.ErrorContext(ctx, "failed to upload media", "error", err)
		return
	}

	a.sendMessage(ctx, target.idType, target.id, imv1.MsgTypeMedia, `{"file_key":"`+mediaKey+`"}`)
}

func (a *LarkAdapter) sendFile(ctx context.Context, target messageTarget, filename string, file []byte) {
	fileKey, err := a.uploadMessageResourceFile(ctx, uploadFileTypeStream, filename, file, nil)
	if err != nil {
		slog.ErrorContext(ctx, "failed to upload file", "error", err)
		return
	}

	a.sendMessage(ctx, target.idType, target.id, imv1.MsgTypeFile, `{"file_key":"`+fileKey+`"}`)
}

const (
	confirmCardInputName      = "extra_text"
	confirmCardButtonNameYes  = "confirm_action_yes"
	confirmCardButtonNameNo   = "confirm_action_no"
	confirmCardButtonValueKey = "confirm_action"
	confirmCardButtonValueYes = "yes"
	confirmCardButtonValueNo  = "no"
)

// TODO: 由于当前是无状态的 重启后就无法回复pending的confirmation了 难以实现 暂时不实现交互式的工具调用确认
func (a *LarkAdapter) buildConfirmationCard(content string, disableButtons bool) *card.CardV2 {
	formContainer := card.NewBodyFormElement("lark-confirm-form")
	descDiv := card.NewBodyDivElement(content)
	input := card.NewBodyInputElement().
		WithName(confirmCardInputName).
		WithPlaceholder("可输入描述").
		WithWidth(card.TextWidthFill)

	confirmButton := card.NewBodyButtonElement("执行").
		WithType(card.ButtonTypePrimary).
		WithName(confirmCardButtonNameYes).
		WithBehavior(&card.Behavior{
			Type: card.BehaviorTypeCallback,
			Value: map[string]string{
				confirmCardButtonValueKey: confirmCardButtonValueYes,
			},
		}).
		WithFormActionType(card.FormActionTypeSubmit)
	rejectButton := card.NewBodyButtonElement("拒绝").
		WithType(card.ButtonTypeDanger).
		WithName(confirmCardButtonNameNo).
		WithBehavior(&card.Behavior{
			Type: card.BehaviorTypeCallback,
			Value: map[string]string{
				confirmCardButtonValueKey: confirmCardButtonValueNo,
			},
		}).
		WithFormActionType(card.FormActionTypeSubmit)
	if disableButtons {
		confirmButton.WithDisabled(true)
		rejectButton.WithDisabled(true)
	}

	buttonCols := card.NewBodyColumnSetElement()
	buttonCols.AddColumn(
		card.NewColumnElement().
			AddElement(confirmButton).
			WithWeight(1)).
		AddColumn(card.NewColumnElement().
			AddElement(rejectButton).
			WithWeight(1),
		)

	formContainer.AddElement(descDiv)
	formContainer.AddElement(input)
	formContainer.AddElement(buttonCols)

	return card.NewCardV2Builder().
		WithHeaderTitle("工具调用确认").
		WithHeaderTemplate(card.HeaderTemplateOrange).
		AppendBodyElement(formContainer).
		Build()
}

func (a *LarkAdapter) sendConfirmationCard(ctx context.Context, target messageTarget, content string) {
	card := a.buildConfirmationCard(content, false)
	cdJson, err := json.Marshal(card)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal card", "error", err)
		return
	}

	a.sendMessage(ctx, target.idType, target.id, imv1.MsgTypeInteractive, xstring.FromBytes(cdJson))
}

func (a *LarkAdapter) replyConfirmationCard(
	ctx context.Context, messageId string, content string,
) (string, error) {
	card := a.buildConfirmationCard(content, false)
	cdJson, err := json.Marshal(card)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal card", "error", err)
		return "", err
	}

	return a.replyMessage(ctx, messageId, imv1.MsgTypeInteractive, xstring.FromBytes(cdJson))
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

func (a *LarkAdapter) replyCard(ctx context.Context, messageId string, content string) (string, error) {
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

func (a *LarkAdapter) replyImage(ctx context.Context, messageId string, image []byte) (string, error) {
	// upload image first
	imgKey, err := a.uploadMessageResourceImage(ctx, image)
	if err != nil {
		return "", fmt.Errorf("failed to upload image: %w", err)
	}

	return a.replyMessage(ctx, messageId, imv1.MsgTypeImage, `{"image_key":"`+imgKey+`"}`)
}

func (a *LarkAdapter) replyAudio(ctx context.Context, messageId string, filename string, audio []byte, audioDuration int) (string, error) {
	audioKey, err := a.uploadMessageResourceFile(
		ctx,
		uploadFileTypeOpus,
		filename,
		audio,
		&messageResourceFileExtra{
			audioDuration: audioDuration,
		})
	if err != nil {
		return "", fmt.Errorf("failed to upload audio: %w", err)
	}

	return a.replyMessage(ctx, messageId, imv1.MsgTypeAudio, `{"file_key":"`+audioKey+`"}`)
}

func (a *LarkAdapter) replyMedia(ctx context.Context, messageId string, filename string, media []byte) (string, error) {
	fileKey, err := a.uploadMessageResourceFile(ctx, uploadFileTypeMp4, filename, media, nil)
	if err != nil {
		return "", fmt.Errorf("failed to upload media: %w", err)
	}

	return a.replyMessage(ctx, messageId, imv1.MsgTypeFile, `{"file_key":"`+fileKey+`"}`)
}

func (a *LarkAdapter) replyFile(ctx context.Context, messageId string, filename string, file []byte) (string, error) {
	var fileType uploadFileType
	switch mime := mimetype.Detect(file).String(); mime {
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		fileType = uploadFileTypeDoc
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		fileType = uploadFileTypeXls
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		fileType = uploadFileTypePpt
	case "application/pdf", "application/x-pdf":
		fileType = uploadFileTypePdf
	default:
		fileType = uploadFileTypeStream
	}

	fileKey, err := a.uploadMessageResourceFile(ctx, fileType, filename, file, nil)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	return a.replyMessage(ctx, messageId, imv1.MsgTypeFile, `{"file_key":"`+fileKey+`"}`)
}

func (a *LarkAdapter) replyCardEntity(ctx context.Context, messageId, cardId string) (string, error) {
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
		slog.ErrorContext(ctx, "failed to send message reaction",
			"error", resp.ErrorResp(), "request_id", resp.RequestId())
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
		content, _, err = a.handleTextMessage(rawContent)
	case imv1.MsgTypePost:
		content, attachments, err = a.handlePostMessage(ctx, rawContent, messageId)
	case imv1.MsgTypeImage:
		var (
			mimeType  string
			imageKey  string
			imageData []byte
		)
		imageKey, imageData, mimeType, err = a.handleImageMessage(ctx, messageId, rawContent)
		if err == nil && len(imageData) > 0 {
			attachments = append(attachments, &model.IncomingMessageAttachment{
				Key:      wrapResourceKey(imageKey),
				Type:     model.AttachmentImage,
				Data:     imageData,
				MimeType: mimeType,
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
