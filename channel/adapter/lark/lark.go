package lark

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/ryanreadbooks/tokkibot/channel/adapter"
	"github.com/ryanreadbooks/tokkibot/channel/adapter/lark/emoji"
	"github.com/ryanreadbooks/tokkibot/channel/model"
	"github.com/ryanreadbooks/tokkibot/pkg/httpx"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	imv1 "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/larksuite/oapi-sdk-go/v3/ws"
)

var _ adapter.Adapter = (*LarkAdapter)(nil)

// feishu
type LarkAdapter struct {
	cli   *lark.Client
	wscli *ws.Client

	input  chan *model.IncomingMessage
	output chan *model.OutgoingMessage

	cancelMu sync.Mutex
	cancels  map[string]context.CancelFunc
}

type LarkConfig struct {
	AppId     string
	AppSecret string
}

func NewAdapter(cfg LarkConfig) *LarkAdapter {
	eventDispatcher := dispatcher.NewEventDispatcher("", "")

	wscli := ws.NewClient(
		cfg.AppId,
		cfg.AppSecret,
		ws.WithLogLevel(larkcore.LogLevelInfo),
		ws.WithEventHandler(eventDispatcher),
		ws.WithAutoReconnect(true),
	)
	cli := lark.NewClient(
		cfg.AppId,
		cfg.AppSecret,
		lark.WithLogLevel(larkcore.LogLevelWarn),
		lark.WithHttpClient(httpx.NewRetryClient(httpx.DefaultRetryConfig())),
	)
	adapter := &LarkAdapter{
		wscli: wscli,
		cli:   cli,

		input:    make(chan *model.IncomingMessage, 1),
		output:   make(chan *model.OutgoingMessage, 16),
		cancels:  make(map[string]context.CancelFunc),
		cancelMu: sync.Mutex{},
	}
	eventDispatcher.OnP2MessageReceiveV1(adapter.onMessageReceive)

	return adapter
}

func (a *LarkAdapter) Type() model.Type {
	return model.Lark
}

func (a *LarkAdapter) ReceiveChan() <-chan *model.IncomingMessage {
	return a.input
}

func (a *LarkAdapter) SendChan() chan<- *model.OutgoingMessage {
	return a.output
}

func (a *LarkAdapter) Start(ctx context.Context) error {
	go a.wscli.Start(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-a.output:
			messageId, _ := msg.Metadata["message_id"].(string)
			reactionId, _ := msg.Metadata["reaction_id"].(string)
			if messageId != "" && reactionId != "" {
				// send new done reaction
				a.sendMessageReaction(ctx, messageId, emoji.DONE)
			}

			// send output message
			a.replyInteractiveMessage(ctx, messageId, msg.Content)

			a.cancelMu.Lock()
			if cancel := a.cancels[messageId]; cancel != nil {
				cancel()
			}
			a.cancelMu.Unlock()
		}
	}
}

func (a *LarkAdapter) onMessageReceive(ctx context.Context, event *imv1.P2MessageReceiveV1) error {
	var (
		// parse message
		senderId       = *event.Event.Sender.SenderId.OpenId
		messageId      = *event.Event.Message.MessageId
		messageType    = *event.Event.Message.MessageType
		chatId         = *event.Event.Message.ChatId
		messageContent = *event.Event.Message.Content
	)

	var (
		content     string
		err         error
		attachments []*model.IncomingMessageAttachment
	)

	switch messageType {
	case imv1.MsgTypeText:
		content, err = a.handleTextMessage(messageContent)
	case imv1.MsgTypePost:
		content, attachments, err = a.handlePostMessage(ctx, messageContent, messageId)
	case imv1.MsgTypeImage:
		var (
			imageData []byte
			imageKey  string
		)
		imageKey, imageData, err = a.handleImageMessage(ctx, messageId, messageContent)
		attachments = append(attachments, &model.IncomingMessageAttachment{
			Key:  wrapResourceKey(imageKey),
			Type: model.AttachmentImage,
			Data: imageData,
		})
	case imv1.MsgTypeFile:
		content = a.handleFileMessage()
	case imv1.MsgTypeAudio:
		content = a.handleAudioMessage()
	case imv1.MsgTypeMedia:
		content = a.handleMediaMessage()
	case imv1.MsgTypeSticker:
		content = a.handleStickerMessage()
	case imv1.MsgTypeInteractive:
		content = a.handleInteractiveMessage()
	case imv1.MsgTypeShareChat:
		content = a.handleShareChatMessage()
	case imv1.MsgTypeShareUser:
		content = a.handleShareUserMessage()
	default:
		err = fmt.Errorf("unsupported lark message type: %s", messageType)
	}

	if err != nil {
		a.sendErrorLog(ctx, senderId, err)
		return nil
	}

	if len(content) == 0 && len(attachments) == 0 {
		slog.InfoContext(ctx, "no content and attachments in lark message", "message_id", messageId)
		return nil
	}

	// reaction to message received
	reactionId := a.sendMessageReaction(ctx, messageId, emoji.Get)

	sourceCtx, sourceCancel := context.WithCancel(ctx)
	a.cancelMu.Lock()
	a.cancels[messageId] = sourceCancel
	a.cancelMu.Unlock()

	// enable streaming mode in lark for better user experience
	streamOutput := make(chan *model.StreamContent)
	incomingMsg := &model.IncomingMessage{
		SenderId:    senderId,
		Channel:     model.Lark,
		ChatId:      chatId,
		Content:     content,
		Attachments: attachments,
		Metadata: map[string]any{
			"message_id":  messageId,
			"reaction_id": reactionId,
		},
		SourceCtx: sourceCtx,
		Stream:    true,
	}
	incomingMsg.SetStreamContent(streamOutput)

	a.input <- incomingMsg

	go a.consumeGatewayStreamMessage(sourceCtx, incomingMsg, streamOutput)

	return nil
}

func (a *LarkAdapter) consumeGatewayStreamMessage(
	ctx context.Context,
	incomingMsg *model.IncomingMessage,
	streamOutput <-chan *model.StreamContent,
) {
	// accumulate content
	var (
		contentBuilder strings.Builder
		seq            int    = 1
		elementId      string = "markdown_1"

		messageId               string = incomingMsg.Metadata["message_id"].(string)
		reactionId              string = incomingMsg.Metadata["reaction_id"].(string)
		streamSendEnabled       bool
		replyMessageId          string
		shouldRecallCardMessage bool
	)

	cardId, err := a.createCardEntityForStream(ctx, elementId)
	slog.InfoContext(ctx, "created card entity for stream", "card_id", cardId, "error", err)
	if err == nil {
		if replyMessageId, err = a.replyInteractiveCardMessage(ctx, messageId, cardId); err == nil {
			slog.InfoContext(ctx, "replied interactive card message", "message_id", messageId, "card_id", cardId)
			streamSendEnabled = true
		}
	} else {
		slog.InfoContext(ctx, "failed to create card entity for stream", "error", err)
	}

	for content := range streamOutput {
		contentBuilder.WriteString(content.Content)
		if streamSendEnabled {
			// update card entity
			err = a.updateCardEntityForStream(ctx, cardId, elementId, contentBuilder.String(), seq)
			if err != nil {
				shouldRecallCardMessage = true
				streamSendEnabled = false // fallback
			}
			seq++
		}
	}

	if !streamSendEnabled {
		if replyMessageId != "" && shouldRecallCardMessage {
			a.recallMessage(ctx, replyMessageId)
		}
		// fallback to normal interactive message
		a.SendChan() <- &model.OutgoingMessage{
			SenderId: incomingMsg.SenderId,
			Channel:  incomingMsg.Channel,
			ChatId:   incomingMsg.ChatId,
			Content:  contentBuilder.String(),
			Metadata: map[string]any{
				"message_id":  messageId,
				"reaction_id": reactionId,
			},
		}
		slog.InfoContext(ctx, "fallback to normal interactive message", "message_id", messageId)
	} else {
		// stop card stream
		a.stopCardEntityStream(ctx, cardId, seq)
		a.sendMessageReaction(ctx, messageId, emoji.DONE)
	}
}
