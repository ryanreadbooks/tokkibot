package model

import "fmt"

type IncomingMessage struct {
	Channel Type
	ChatId  string
	Created int64  // unix timestamp in seconds
	Content string // message text
}

// unique session identifier for the message
func (m *IncomingMessage) SessionKey() string {
	return fmt.Sprintf("%s:%s", m.Channel, m.ChatId)
}

type OutgoingMessage struct {
	Channel Type
	ChatId  string
	Created int64 // unix timestamp in seconds
	Content string
}
