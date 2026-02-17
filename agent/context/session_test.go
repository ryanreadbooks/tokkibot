package context

import (
	"encoding/json"
	"strings"
	"testing"

	llmmodel "github.com/ryanreadbooks/tokkibot/llm/model"
)

func TestAssistantCompact(t *testing.T) {
	tcs := []llmmodel.CompletionToolCall{
		{
			Function: llmmodel.CompletionToolCallFunction{
				Arguments: strings.Repeat("abc", 60),
			},
		},
	}
	logItem := SessionLogItem{
		Role:   llmmodel.RoleAssistant,
		Extras: map[string]any{extraToolCallsKey: tcs},
	}

	compactThreshold = 60

	logItem.compactAssistant()

	debug, _ := json.Marshal(logItem.Extras)
	t.Log(string(debug))
}
