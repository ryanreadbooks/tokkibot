package estimator

import (
	"testing"

	"github.com/ryanreadbooks/tokkibot/llm/schema"
	"github.com/ryanreadbooks/tokkibot/llm/schema/param"
)

func TestEstimateToken(t *testing.T) {
	text := "Hello world, are you ok?"
	t.Log(EstimateToken(text))
}

func TestEstimateRequestToken(t *testing.T) {
	est, _ := RoughEstimator{}.Estimate(t.Context(),
		&schema.Request{
			Messages: []param.Message{
				param.NewSystemMessage("Hello, you are a smart agent"),
				param.NewUserMessage("What are your skills"),
				param.NewAssistantMessage("Wonderful",
					[]*param.ToolCall{
						{
							Function: &param.ToolCallFunction{
								Id:        "qwoi",
								Name:      "abc",
								Arguments: "{\"name\": \"ryan\", \"age\": 19}",
							},
						},
					},
					&param.String{
						Value: "Let me think",
					}),
			},
			Tools: []param.Tool{
				{
					Definition: &param.ToolDefinition{
						Name:        "qeo",
						Description: "this is a tool",
					},
					Parameters: map[string]any{
						"name": map[string]any{
							"properties": 190,
							"required":   []string{"name"},
						},
					},
				},
			},
		},
	)

	t.Log(est)
}
