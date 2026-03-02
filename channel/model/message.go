package model

import (
	"context"
	"fmt"
)

// Stream content for streaming response.
type StreamContent struct {
	Content          string
	ReasoningContent string
}

type StreamTool struct {
	Name      string
	Arguments string
}

type IncomingMessage struct {
	SenderId    string
	Channel     Type
	ChatId      string
	Created     int64  // unix timestamp in seconds
	Content     string // message text
	Attachments []*IncomingMessageAttachment
	Metadata    map[string]any

	// req params passed to gateway
	SourceCtx     context.Context
	Stream        bool
	streamContent chan *StreamContent // output content receiving
	streamTool    chan *StreamTool    // output tool call receiving
}

func (m *IncomingMessage) SetStreamContent(ch chan *StreamContent) {
	m.streamContent = ch
}

func (m *IncomingMessage) SetStreamTool(ch chan *StreamTool) {
	m.streamTool = ch
}

func (m *IncomingMessage) StreamContent() chan<- *StreamContent {
	return m.streamContent
}

func (m *IncomingMessage) StreamTool() chan<- *StreamTool {
	return m.streamTool
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
