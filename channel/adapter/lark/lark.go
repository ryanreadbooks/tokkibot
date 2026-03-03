package lark

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

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
			messageId, _ := msg.Metadata[metaKeyMessageId].(string)
			reactionId, _ := msg.Metadata[metaKeyReactionId].(string)
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

// derefStr safely dereferences a string pointer, returning empty string if nil
func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (a *LarkAdapter) onMessageReceive(ctx context.Context, event *imv1.P2MessageReceiveV1) error {
	if event.Event == nil || event.Event.Sender == nil || event.Event.Message == nil {
		slog.ErrorContext(ctx, "invalid message event", "event", event)
		return nil
	}

	msg := event.Event.Message
	sender := event.Event.Sender

	// safely extract sender id
	var senderId string
	if sender.SenderId != nil {
		senderId = derefStr(sender.SenderId.OpenId)
	}

	// safely extract message fields
	messageId := derefStr(msg.MessageId)
	messageType := derefStr(msg.MessageType)
	chatId := derefStr(msg.ChatId)
	messageContent := derefStr(msg.Content)
	parentId := derefStr(msg.ParentId)

	// validate required fields
	if senderId == "" || messageId == "" {
		slog.ErrorContext(ctx, "missing required message fields", "sender_id", senderId, "message_id", messageId)
		return nil
	}

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

	// generate placeholders for current message's attachments
	if len(attachments) > 0 {
		var placeholders []string
		for i, att := range attachments {
			placeholders = append(placeholders, fmt.Sprintf("[%s-%d]", att.Type, i+1))
		}
		if content == "" {
			content = strings.Join(placeholders, " ")
		} else {
			content = fmt.Sprintf("%s %s", content, strings.Join(placeholders, " "))
		}
	}

	// handle quoted messages: wrap each quoted content with <quote> tags
	if parentId != "" {
		if quotedList := a.getQuotedMessages(ctx, parentId); len(quotedList) > 0 {
			for _, quoted := range quotedList {
				quotedContent := quoted.Content
				// generate placeholders for quoted message's attachments
				if len(quoted.Attachments) > 0 {
					var placeholders []string
					for i, att := range quoted.Attachments {
						idx := len(attachments) + i + 1
						placeholders = append(placeholders, fmt.Sprintf("[%s-%d]", att.Type, idx))
					}
					if quotedContent == "" {
						quotedContent = strings.Join(placeholders, " ")
					} else {
						quotedContent = fmt.Sprintf("%s %s", quotedContent, strings.Join(placeholders, " "))
					}
				}
				if quotedContent != "" {
					content = fmt.Sprintf("%s\n\n<quote>\n%s\n</quote>", content, quotedContent)
				}
				attachments = append(attachments, quoted.Attachments...)
			}
		}
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

	// stream state for callbacks (init is delayed to first onContent)
	state := &larkStreamState{
		adapter:   a,
		ctx:       sourceCtx,
		messageId: messageId,
		elementId: "markdown_1",
		seq:       1,
	}

	incomingMsg := &model.IncomingMessage{
		SenderId:    senderId,
		Channel:     model.Lark,
		ChatId:      chatId,
		Content:     content,
		Attachments: attachments,
		Metadata: map[string]any{
			metaKeyMessageId:  messageId,
			metaKeyReactionId: reactionId,
		},
		SourceCtx: sourceCtx,
		Stream:    true,
		OnContent: state.onContent,
		OnDone:    state.onDone,
	}

	a.input <- incomingMsg

	return nil
}

const (
	streamFlushInterval  = 300 * time.Millisecond
	streamFlushThreshold = 256 // flush when accumulated content exceeds this

	metaKeyMessageId  = "message_id"
	metaKeyReactionId = "reaction_id"
)

type larkStreamState struct {
	adapter   *LarkAdapter
	ctx       context.Context
	messageId string
	elementId string

	initOnce                sync.Once
	mu                      sync.Mutex
	contentBuilder          strings.Builder
	reasoningContentBuilder strings.Builder
	seq                     int
	cardId                  string
	replyMessageId          string
	streamSendEnabled       bool
	shouldRecallCardMessage bool

	dirty      bool
	lastSeqLen int
	stopCh     chan struct{}
}

func (s *larkStreamState) init() {
	cardId, err := s.adapter.createCardEntityForStream(s.ctx, s.elementId)
	slog.InfoContext(s.ctx, "created card entity for stream", "card_id", cardId, "error", err)
	if err == nil {
		replyMessageId, err := s.adapter.replyInteractiveCardMessage(s.ctx, s.messageId, cardId)
		if err == nil {
			slog.InfoContext(s.ctx, "replied interactive card message", "message_id", s.messageId, "card_id", cardId)
			s.cardId = cardId
			s.replyMessageId = replyMessageId
			s.streamSendEnabled = true
			s.stopCh = make(chan struct{})
			go s.flushLoop()
		}
	} else {
		slog.InfoContext(s.ctx, "failed to create card entity for stream", "error", err)
	}
}

func (s *larkStreamState) flushLoop() {
	ticker := time.NewTicker(streamFlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.flush()
		}
	}
}

func (s *larkStreamState) flush() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.streamSendEnabled || !s.dirty {
		return
	}

	content := s.contentBuilder.String()
	if len(content) == s.lastSeqLen {
		return
	}

	err := s.adapter.updateCardEntityForStream(s.ctx, s.cardId, s.elementId, content, s.seq)
	if err != nil {
		s.shouldRecallCardMessage = true
		s.streamSendEnabled = false
		return
	}
	s.seq++
	s.lastSeqLen = len(content)
	s.dirty = false
}

func (s *larkStreamState) onContent(content *model.StreamContent) {
	s.initOnce.Do(s.init)

	s.mu.Lock()
	s.contentBuilder.WriteString(content.Content)
	s.reasoningContentBuilder.WriteString(content.ReasoningContent)
	s.dirty = true
	pendingLen := s.contentBuilder.Len() - s.lastSeqLen
	s.mu.Unlock()

	if pendingLen >= streamFlushThreshold {
		s.flush()
	}
}

func (s *larkStreamState) onDone() {
	if s.stopCh != nil {
		close(s.stopCh)
	}

	s.flush()

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.streamSendEnabled {
		if s.replyMessageId != "" && s.shouldRecallCardMessage {
			s.adapter.recallMessage(s.ctx, s.replyMessageId)
		}
		s.adapter.replyInteractiveMessage(s.ctx, s.messageId, s.contentBuilder.String())
		slog.InfoContext(s.ctx, "fallback to normal interactive message", "message_id", s.messageId)
	} else {
		s.adapter.stopCardEntityStream(s.ctx, s.cardId, s.seq)
		s.adapter.sendMessageReaction(s.ctx, s.messageId, emoji.DONE)
	}
}
