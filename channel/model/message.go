package model

import (
	"context"
	"fmt"
)

// Stream content for streaming response.
type StreamContent struct {
	Round            int
	Content          string
	ReasoningContent string
}

type StreamTool struct {
	Round     int
	Name      string
	Arguments string
}

// Callback handlers for streaming
type StreamContentHandler func(*StreamContent)
type StreamToolHandler func(*StreamTool)
type StreamDoneHandler func()

type IncomingMessage struct {
	SenderId    string
	Channel     Type
	ChatId      string
	Created     int64  // unix timestamp in seconds
	Content     string // message text
	Attachments []*IncomingMessageAttachment
	Metadata    map[string]any

	// req params passed to gateway
	SourceCtx context.Context

	// Enable streaming response
	Stream    bool

	// Stream callbacks - adapter implements these
	OnContent StreamContentHandler
	OnTool    StreamToolHandler
	OnDone    StreamDoneHandler
}

func (m *IncomingMessage) EmitContent(content *StreamContent) {
	if m.OnContent != nil {
		m.OnContent(content)
	}
}

func (m *IncomingMessage) EmitTool(tool *StreamTool) {
	if m.OnTool != nil {
		m.OnTool(tool)
	}
}

func (m *IncomingMessage) EmitDone() {
	if m.OnDone != nil {
		m.OnDone()
	}
}

func (m *IncomingMessage) Context() context.Context {
	if m.SourceCtx == nil {
		return context.Background()
	}

	return m.SourceCtx
}

type AttachmentType string

const (
	AttachmentImage AttachmentType = "image"
	AttachmentFile  AttachmentType = "file"
	AttachmentVideo AttachmentType = "video"
)

type IncomingMessageAttachment struct {
	Key  string
	Type AttachmentType
	Data []byte
}

// unique session identifier for the message
func (m *IncomingMessage) Key() string {
	return fmt.Sprintf("%s:%s", m.Channel, m.ChatId)
}

// TODO add more field
type OutgoingMessage struct {
	SenderId string
	Channel  Type
	ChatId   string
	Content  string
	Metadata map[string]any
}
