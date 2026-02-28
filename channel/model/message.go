package model

import "fmt"

type IncomingMessage struct {
	SenderId string
	Channel  Type
	ChatId   string
	Created  int64  // unix timestamp in seconds
	Content  string // message text
	Metadata map[string]any
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
