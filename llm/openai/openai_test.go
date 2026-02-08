package openai

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/ryanreadbooks/tokkibot/llm"
	"github.com/ryanreadbooks/tokkibot/llm/model"
)

type GetWeatherInput struct {
	Lat  float64 `json:"lat" jsonschema:"description=The latitude of the location to check weather"`
	Long float64 `json:"long" jsonschema:"description=The longitude of the location to check weather"`
}

type GetWeatherResponse struct {
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	CurrentWeather struct {
		Time          string  `json:"time"`
		Interval      int     `json:"interval"`
		Temperature   float64 `json:"temperature"`
		Windspeed     float64 `json:"windspeed"`
		Winddirection int     `json:"winddirection"`
		IsDay         int     `json:"is_day"`
		Weathercode   int     `json:"weathercode"`
	} `json:"current_weather"`
}

// https://api.open-meteo.com/v1/forecast?latitude=39.9042&longitude=116.4074&current_weather=true
func GetWeather(lat, long float64) (string, error) {
	url := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current_weather=true", lat, long)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var response GetWeatherResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("The weather in %f, %f is %f°C", lat, long, response.CurrentWeather.Temperature), nil
}

func TestChatCompletion(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")

	if apiKey == "" || baseURL == "" {
		t.Fatalf("OPENAI_API_KEY and OPENAI_BASE_URL are required")
	}

	openAi, err := New(Config{
		ApiKey:  apiKey,
		BaseURL: baseURL,
	})
	if err != nil {
		t.Fatalf("Failed to create LLM: %v", err)
	}

	messages := []model.MessageParam{
		model.NewSystemMessageParam("You are a helpful assistant."),
		model.NewUserMessageParam(
			"Please get the weather in Shanghai, China, use approximate latitude and longitude, and return the weather in the format of 'The weather in xx is yy°C'"),
	}

	tools := []model.ToolParam{
		model.NewToolParam[GetWeatherInput]("get_weather", "Get the weather for a given location"),
	}

	ctx := t.Context()
	defaultModel := "kimi-k2-0905-preview"

	maxRounds := 10

	usage := model.CompletionUsage{}

	for round := 0; round < maxRounds; round++ {
		resp, err := openAi.ChatCompletion(ctx, &llm.Request{
			Model:       defaultModel,
			Temperature: 1.0,
			Messages:    messages,
			Tools:       tools,
		})
		if err != nil {
			t.Fatalf("Failed to chat completion: %v", err)
		}

		usage.CompletionTokens += resp.Usage.CompletionTokens
		usage.PromptTokens += resp.Usage.PromptTokens
		usage.TotalTokens += resp.Usage.TotalTokens

		choice := resp.Choices[0]
		fmt.Printf("Round %d:\n", round+1)
		fmt.Printf("Content: %s\n", choice.Message.Content)

		if choice.Message.HasToolCalls() {
			fmt.Println("Tool Calls:")
			messages = append(messages,
				model.NewAssistantMessageParam(choice.Message.Content, choice.Message.GetToolCallParams()))

			for _, toolCall := range choice.Message.ToolCalls {
				fmt.Println(toolCall.Id)
				fmt.Println(toolCall.Function.Name)
				fmt.Println(toolCall.Function.Arguments)

				// add assistant message to messages
				// simulate append tool result to messages
				var input GetWeatherInput
				err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input)
				if err != nil {
					messages = append(messages, model.NewToolMessageParam(toolCall.Id, fmt.Sprintf("Failed to unmarshal arguments: %v", err)))
				} else {
					result, err := GetWeather(input.Lat, input.Long)
					if err != nil {
						messages = append(messages, model.NewToolMessageParam(toolCall.Id, fmt.Sprintf("Failed to get weather: %v", err)))
					} else {
						messages = append(messages, model.NewToolMessageParam(toolCall.Id, result))
					}
				}
			}
		} else {
			fmt.Println("Final result:")
			fmt.Println(resp.Choices[0].Message.Content)
			fmt.Printf("Usage: %+v\n", usage)
			break
		}
	}
}
