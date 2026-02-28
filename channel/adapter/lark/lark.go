package lark

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ryanreadbooks/tokkibot/channel/adapter"
	"github.com/ryanreadbooks/tokkibot/channel/adapter/lark/emoji"
	"github.com/ryanreadbooks/tokkibot/channel/model"

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
		ws.WithLogLevel(larkcore.LogLevelWarn),
		ws.WithEventHandler(eventDispatcher))
	cli := lark.NewClient(
		cfg.AppId,
		cfg.AppSecret,
		lark.WithLogLevel(larkcore.LogLevelWarn))
	adapter := &LarkAdapter{
		wscli: wscli,
		cli:   cli,

		input:  make(chan *model.IncomingMessage, 128),
		output: make(chan *model.OutgoingMessage, 128),
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
		content string
		err     error
	)

	switch messageType {
	case imv1.MsgTypeText:
		content, err = a.handleTextMessage(messageContent)
	case imv1.MsgTypePost:
		content, err = a.handlePostMessage(messageContent)
	case imv1.MsgTypeImage:
		content = a.handleImageMessage()
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

	if len(content) == 0 {
		slog.InfoContext(ctx, "no content in lark message", "message_id", messageId)
		return nil
	}

	// reaction to message received
	reactionId := a.sendMessageReaction(ctx, messageId, emoji.Get)

	a.input <- &model.IncomingMessage{
		SenderId: senderId,
		Channel:  model.Lark,
		ChatId:   chatId,
		Content:  content,
		Metadata: map[string]any{
			"message_id":  messageId,
			"reaction_id": reactionId,
		},
	}

	return nil
}
