package schema

import "encoding/json"

type ThinkingType string

const (
	ThinkingTypeEnabled  ThinkingType = "enabled"
	ThinkingTypeDisabled ThinkingType = "disabled"
)

type Thinking struct {
	Type ThinkingType `json:"type"`
}

func (t *Thinking) Json() string {
	s, _ := json.Marshal(t)
	return string(s)
}

func EnableThinking() *Thinking {
	return &Thinking{
		Type: ThinkingTypeEnabled,
	}
}

func DisableThinking() *Thinking {
	return &Thinking{
		Type: ThinkingTypeDisabled,
	}
}
