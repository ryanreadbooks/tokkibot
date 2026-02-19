package estimator

import (
	"testing"

	"github.com/ryanreadbooks/tokkibot/llm/schema"
)

func TestEstimateToken(t *testing.T) {
	text := "Hello world, are you ok?"
	t.Log(EstimateToken(text))
}

func TestEstimateRequestToken(t *testing.T) {
	est, _ := RoughEstimator{}.Estimate(t.Context(),
		&schema.Request{
			Messages: []schema.MessageParam{
				schema.NewSystemMessageParam("Hello, you are a smart agent"),
				schema.NewUserMessageParam("What are your skills"),
				schema.NewAssistantMessageParam("Wonderful",
					[]*schema.ToolCallParam{
						{
							Function: &schema.ToolCallFunctionParam{
								Id:        "qwoi",
								Name:      "abc",
								Arguments: "{\"name\": \"ryan\", \"age\": 19}",
							},
						},
					},
					&schema.StringParam{
						Value: "Let me think",
					}),
			},
			Tools: []schema.ToolParam{
				{
					Definition: &schema.ToolDefinitionParam{
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
