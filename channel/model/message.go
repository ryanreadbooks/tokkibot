package model

import (
	"context"
	"fmt"
)

type IncomingMessage struct {
	SenderId    string
	Channel     Type
	ChatId      string
	Created     int64  // unix timestamp in seconds
	Content     string // message text
	Attachments []*IncomingMessageAttachment
	Metadata    map[string]any

	SourceCtx context.Context
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
