package anthropic

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/ryanreadbooks/tokkibot/llm/schema"
	"github.com/ryanreadbooks/tokkibot/llm/schema/param"
)

func TestAnthropicChatCompletion(t *testing.T) {
	an, err := New(Config{
		ApiKey:  os.Getenv("ANTHROPIC_API_KEY"),
		BaseURL: os.Getenv("ANTHROPIC_BASE_URL"),
	})
	if err != nil {
		t.Fatalf("Failed to create anthropic client: %v", err)
	}

	ctx := t.Context()
	messages := []param.Message{
		param.NewSystemMessage("You are a helpful assistant."),
		param.NewUserMessage("What is the weather in Shanghai?"),
	}

	type getWeatherInput struct {
		City string `json:"city" jsonschema:"description=The city to get the weather of"`
	}

	req := schema.NewRequest("deepseek-reasoner", messages)
	req.Tools = append(req.Tools, param.NewTool[getWeatherInput](
		"get_weather", "Get the weather for a given location"))

	resp, err := an.ChatCompletion(ctx, req)
	if err != nil {
		t.Fatalf("Failed to chat completion: %v", err)
	}

	output, _ := json.MarshalIndent(resp, "", " ")
	t.Log(string(output))
}
