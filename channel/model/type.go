package model

type Type string

func (t Type) String() string { return string(t) }

const (
	ChannelCLI Type = "cli"
)
