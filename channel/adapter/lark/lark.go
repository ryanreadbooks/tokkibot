package lark

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
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

type slogAdapter struct {
	level larkcore.LogLevel
}

func (s *slogAdapter) Debug(ctx context.Context, args ...any) {
	if s.level > larkcore.LogLevelDebug {
		return
	}
	slog.DebugContext(ctx, "[lark-sdk]", "msg", fmt.Sprint(args...))
}

func (s *slogAdapter) Info(ctx context.Context, args ...any) {
	if s.level > larkcore.LogLevelInfo {
		return
	}
	slog.InfoContext(ctx, "[lark-sdk]", "msg", fmt.Sprint(args...))
}

func (s *slogAdapter) Warn(ctx context.Context, args ...any) {
	if s.level > larkcore.LogLevelWarn {
		return
	}
	slog.WarnContext(ctx, "[lark-sdk]", "msg", fmt.Sprint(args...))
}

func (s *slogAdapter) Error(ctx context.Context, args ...any) {
	if s.level > larkcore.LogLevelError {
		return
	}
	slog.ErrorContext(ctx, "[lark-sdk]", "msg", fmt.Sprint(args...))
}

// feishu
type LarkAdapter struct {
	cfg   LarkConfig
	cli   *lark.Client
	wscli *ws.Client

	input  chan *model.IncomingMessage
	output chan *model.OutgoingMessage

	cancelMu sync.Mutex
	cancels  map[string]context.CancelFunc

	pendingConfirmEvtsMu sync.Mutex
	pendingConfirmEvts   map[string]*model.ConfirmEvent

	botOpenIdMu sync.RWMutex
	botOpenId   string
}

type LarkConfig struct {
	AppId          string `json:"appId"`
	AppSecret      string `json:"appSecret"`
	RequireMention bool   `json:"requireMention"` // 当机器人处于群聊中时 只有@机器人时才处理消息
}

func NewAdapter(cfg LarkConfig) *LarkAdapter {
	eventDispatcher := dispatcher.NewEventDispatcher("", "")

	// Create slog-based logger for lark SDK
	logger := &slogAdapter{
		level: larkcore.LogLevelWarn,
	}

	wscli := ws.NewClient(
		cfg.AppId,
		cfg.AppSecret,
		ws.WithLogLevel(logger.level),
		ws.WithLogger(logger),
		ws.WithEventHandler(eventDispatcher),
		ws.WithAutoReconnect(true),
	)
	cli := lark.NewClient(
		cfg.AppId,
		cfg.AppSecret,
		lark.WithLogLevel(logger.level),
		lark.WithLogger(logger),
		lark.WithHttpClient(httpx.NewRetryClient(httpx.DefaultRetryConfig())),
	)
	adapter := &LarkAdapter{
		wscli:   wscli,
		cli:     cli,
		cfg:     cfg,
		input:   make(chan *model.IncomingMessage, 1),
		output:  make(chan *model.OutgoingMessage, 16),
		cancels: make(map[string]context.CancelFunc),

		pendingConfirmEvts: make(map[string]*model.ConfirmEvent),
	}
	eventDispatcher.OnP2MessageReceiveV1(adapter.onMessageReceive)

	// init get openid
	botOpenId, err := adapter.GetBotOpenId(context.Background())
	if err != nil {
		slog.ErrorContext(context.Background(), "failed to get bot open id", "error", err)
	} else {
		adapter.botOpenIdMu.Lock()
		adapter.botOpenId = botOpenId
		adapter.botOpenIdMu.Unlock()
		slog.Info("bot open id", "bot_open_id", botOpenId)
	}

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
			a.onOutgoingMessage(ctx, msg)
		}
	}
}

func (a *LarkAdapter) onOutgoingMessage(ctx context.Context, msg *model.OutgoingMessage) {
	messageId, _ := msg.Metadata[metaKeyMessageId].(string)

	if messageId != "" { // reply message
		if msg.Content != "" {
			a.replyCard(ctx, messageId, msg.Content)
		}
		if len(msg.Attachments) > 0 {
			a.doReplyAttachments(ctx, messageId, msg.Attachments)
		}
	} else if msg.ReceiverId != "" { // send message directly
		var target messageTarget
		if strings.HasPrefix(msg.ReceiverId, "ou_") {
			target = userTarget(msg.ReceiverId)
		} else {
			target = chatTarget(msg.ReceiverId)
		}
		if msg.Content != "" {
			a.sendCard(ctx, target, msg.Content)
		}
		if len(msg.Attachments) > 0 {
			a.doSendAttachments(ctx, target, msg.Attachments)
		}
	}
}

func (a *LarkAdapter) doReplyAttachments(ctx context.Context, messageId string, attachments []*model.OutgoingMessageAttachment) {
	for _, attachment := range attachments {
		switch attachment.Type {
		case model.AttachmentImage:
			a.replyImage(ctx, messageId, attachment.Data)
		case model.AttachmentAudio:
			a.replyAudio(ctx, messageId, attachment.Filename, attachment.Data)
		case model.AttachmentVideo:
			a.replyMedia(ctx, messageId, attachment.Filename, attachment.Data)
		case model.AttachmentFile:
			a.replyFile(ctx, messageId, attachment.Filename, attachment.Data)
		}
	}
}

func (a *LarkAdapter) doSendAttachments(ctx context.Context, target messageTarget, attachments []*model.OutgoingMessageAttachment) {
	for _, attachment := range attachments {
		switch attachment.Type {
		case model.AttachmentImage:
			a.sendImage(ctx, target, attachment.Data)
		case model.AttachmentAudio:
			a.sendAudio(ctx, target, attachment.Filename, attachment.Data)
		case model.AttachmentVideo:
			a.sendMedia(ctx, target, attachment.Filename, attachment.Data)
		case model.AttachmentFile:
			a.sendFile(ctx, target, attachment.Filename, attachment.Data)
		}
	}
}

func (a *LarkAdapter) onMessageReceive(ctx context.Context, event *imv1.P2MessageReceiveV1) error {
	if event.Event == nil || event.Event.Sender == nil || event.Event.Message == nil {
		slog.WarnContext(ctx, "invalid message event", "event", event)
		return nil
	}

	var (
		senderId string
		msg      = event.Event.Message
		sender   = event.Event.Sender
	)

	// safely extract sender id
	if sender.SenderId != nil {
		senderId = derefStr(sender.SenderId.OpenId)
	}

	var (
		// safely extract message fields
		messageId      = derefStr(msg.MessageId)
		messageType    = derefStr(msg.MessageType)
		chatId         = derefStr(msg.ChatId)
		messageContent = derefStr(msg.Content) // of json format
		parentId       = derefStr(msg.ParentId)
		chatType       = derefStr(msg.ChatType) // p2p, group, topic_group
	)

	// validate required fields
	if senderId == "" || messageId == "" {
		slog.WarnContext(ctx, "missing required message fields", "sender_id", senderId, "message_id", messageId)
		return nil
	}

	if chatType == "group" && a.cfg.RequireMention {
		botOpenId, err := a.GetBotOpenId(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get bot open id", "error", err)
			return nil
		}

		amIMentioned := false
		for _, mention := range event.Event.Message.Mentions {
			if mention.Id != nil && derefStr(mention.Id.OpenId) == botOpenId {
				amIMentioned = true
				break
			}
		}

		if !amIMentioned {
			return nil
		}
	}

	var (
		content     string
		err         error
		attachments []*model.IncomingMessageAttachment
	)

	switch messageType {
	case imv1.MsgTypeText:
		content, _, err = a.handleTextMessage(messageContent)
	case imv1.MsgTypePost:
		content, attachments, err = a.handlePostMessage(ctx, messageContent, messageId)
	case imv1.MsgTypeImage:
		var (
			imageData []byte
			imageKey  string
			mimeType  string
		)
		imageKey, imageData, mimeType, err = a.handleImageMessage(ctx, messageId, messageContent)
		attachments = append(attachments, &model.IncomingMessageAttachment{
			Key:      wrapResourceKey(imageKey),
			Type:     model.AttachmentImage,
			Data:     imageData,
			MimeType: mimeType,
		})
	case imv1.MsgTypeFile:
		var (
			fileData []byte
			fileName string
		)
		fileName, fileData, err = a.handleFileMessage(ctx, messageId, messageContent)
		attachments = append(attachments, &model.IncomingMessageAttachment{
			Key:  wrapResourceKey(fileName),
			Type: model.AttachmentFile,
			Data: fileData,
		})
	case imv1.MsgTypeAudio:
		var (
			audioData []byte
			audioKey  string
		)
		audioKey, audioData, err = a.handleAudioMessage(ctx, messageId, messageContent)
		attachments = append(attachments, &model.IncomingMessageAttachment{
			Key:  wrapResourceKey(audioKey),
			Type: model.AttachmentAudio,
			Data: audioData,
		})
	case imv1.MsgTypeMedia:
		content, err = a.handleMediaMessage()
	case imv1.MsgTypeSticker:
		content, err = a.handleStickerMessage()
	case imv1.MsgTypeInteractive:
		content = a.handleInteractiveMessage(messageContent)
	case imv1.MsgTypeShareChat:
		content, err = a.handleShareChatMessage(ctx, messageId, messageContent)
	case imv1.MsgTypeShareUser:
		content, err = a.handleShareUserMessage(ctx, messageId, messageContent)
	case "location":
		content, err = a.handleLocationMessage(ctx, messageId, messageContent)
	default:
		err = fmt.Errorf("[%s] 我暂时还看不懂这个消息: %s", emoji.ClownFace, messageType)
	}

	if err != nil {
		slog.ErrorContext(ctx, "failed to handle lark message",
			"message_id", messageId,
			"message_type", messageType,
			"content", messageContent,
			"attachments", len(attachments),
			"error", err)
		a.sendError(ctx, senderId, err)
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
	reactionId := a.sendMessageReaction(ctx, messageId, emoji.Typing)

	sourceCtx, sourceCancel := context.WithCancel(ctx)
	a.cancelMu.Lock()
	a.cancels[messageId] = sourceCancel
	a.cancelMu.Unlock()

	// stream state for callbacks (init is delayed to first onContent)
	state := &larkStreamState{
		adapter:                   a,
		ctx:                       sourceCtx,
		messageId:                 messageId,
		reactionId:                reactionId,
		contentElementId:          "markdown_1",
		reasoningContentElementId: "reasoning_markdown_1",
		seq:                       1,
	}

	incomingMsg := &model.IncomingMessage{
		SenderId:    senderId,
		Channel:     model.Lark,
		ChatId:      chatId,
		Content:     content,
		Attachments: attachments,
		Metadata: map[string]any{
			metaKeyMessageId:  messageId,
			metaKeySenderId:   senderId,
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
	metaKeySenderId   = "sender_id"
	metaKeyReactionId = "reaction_id"
)

type larkStreamState struct {
	adapter    *LarkAdapter
	ctx        context.Context
	messageId  string
	reactionId string

	thinkingFinished atomic.Bool
	thinkingEnabled  bool

	reasoningContentElementId string
	contentElementId          string

	initOnce                sync.Once
	mu                      sync.Mutex
	contentBuilder          strings.Builder
	reasoningContentBuilder strings.Builder
	seq                     int
	cardId                  string
	replyMessageId          string
	streamSendEnabled       bool
	shouldRecallCardMessage bool

	dirty               bool
	lastSeqLen          int
	lastReasoningSeqLen int
	stopCh              chan struct{}
}

func (s *larkStreamState) init(thinkingEnaled bool) {
	s.thinkingEnabled = thinkingEnaled
	// we have to create the card entity first for later streaming update
	cardId, err := s.adapter.createCardEntityForStream(s.ctx,
		s.contentElementId,
		s.reasoningContentElementId,
		thinkingEnaled,
	)
	slog.InfoContext(s.ctx, "created card entity for stream", "card_id", cardId, "error", err)
	if err == nil {
		replyMessageId, err := s.adapter.replyCardEntity(s.ctx, s.messageId, cardId)
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
			s.flush(s.ctx)
		}
	}
}

func (s *larkStreamState) flush(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.streamSendEnabled || !s.dirty {
		return
	}

	content := s.contentBuilder.String()
	reasoningContent := s.reasoningContentBuilder.String()

	contentChanged := len(content) != s.lastSeqLen
	reasoningChanged := len(reasoningContent) != s.lastReasoningSeqLen
	if !contentChanged && !reasoningChanged {
		return
	}

	if s.thinkingEnabled && reasoningChanged && len(reasoningContent) > 0 {
		s.adapter.updateCardEntityForStream(ctx,
			s.cardId, s.reasoningContentElementId,
			reasoningContent,
			s.seq)
		s.seq++
		s.lastReasoningSeqLen = len(reasoningContent)
	}

	if s.thinkingEnabled && s.thinkingFinished.Load() {
		s.adapter.patchThinkingElementToFinished(ctx, s.cardId, s.seq)
		s.seq++
	}

	if contentChanged && len(content) > 0 {
		err := s.adapter.updateCardEntityForStream(ctx, s.cardId, s.contentElementId, content, s.seq)
		if err != nil {
			s.shouldRecallCardMessage = true
			s.streamSendEnabled = false
			return
		}
		s.seq++
		s.lastSeqLen = len(content)
	}

	s.dirty = false
}

func (s *larkStreamState) onContent(content *model.StreamContent) {
	s.initOnce.Do(func() { s.init(content.ThinkingEnabled) })

	oldContent := s.contentBuilder.String()
	s.mu.Lock()
	s.contentBuilder.WriteString(content.Content)
	s.reasoningContentBuilder.WriteString(content.ReasoningContent)
	s.dirty = true
	pendingContentLen := s.contentBuilder.Len() - s.lastSeqLen
	pendingReasoningLen := s.reasoningContentBuilder.Len() - s.lastReasoningSeqLen
	s.mu.Unlock()

	newContent := s.contentBuilder.String()
	if oldContent == "" && newContent != "" && content.ThinkingEnabled {
		s.thinkingFinished.Store(true)
	}

	if pendingContentLen >= streamFlushThreshold || pendingReasoningLen >= streamFlushThreshold {
		s.flush(s.ctx)
	}
}

func (s *larkStreamState) onDone() {
	if s.stopCh != nil {
		close(s.stopCh)
	}

	// sourceCtx may already be canceled (e.g. send_message tool triggers
	// onOutgoingMessage which cancels sourceCtx before the agent finishes).
	// Use a non-cancelable context so cleanup API calls always go through.
	cleanupCtx := context.WithoutCancel(s.ctx)

	s.thinkingFinished.Store(true)
	s.flush(cleanupCtx)

	s.mu.Lock()
	if !s.streamSendEnabled {
		if s.replyMessageId != "" && s.shouldRecallCardMessage {
			s.adapter.recallMessage(cleanupCtx, s.replyMessageId)
		}
		s.adapter.replyCard(cleanupCtx, s.messageId, s.contentBuilder.String())
		slog.InfoContext(cleanupCtx, "fallback to normal interactive message", "message_id", s.messageId)
	} else {
		s.adapter.stopCardEntityStream(cleanupCtx, s.cardId, s.seq)
	}
	s.mu.Unlock()

	s.adapter.deleteMessageReaction(cleanupCtx, s.messageId, s.reactionId)

	// cancel sourceCtx and clean up — this is the right place since the agent is truly done
	s.adapter.cancelMu.Lock()
	if cancel := s.adapter.cancels[s.messageId]; cancel != nil {
		cancel()
		delete(s.adapter.cancels, s.messageId)
	}
	s.adapter.cancelMu.Unlock()
}
