package model

type Type string

func (t Type) String() string { return string(t) }

const (
	CLI  Type = "cli"
	Lark Type = "lark" // feishu
)
