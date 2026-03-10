package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"

	"github.com/ryanreadbooks/tokkibot/llm/schema"
	"github.com/ryanreadbooks/tokkibot/llm/schema/param"
)

var testOpenAi *OpenAI

type testGetWeatherInput struct {
	Lat  float64 `json:"lat" jsonschema:"description=The latitude of the location to check weather"`
	Long float64 `json:"long" jsonschema:"description=The longitude of the location to check weather"`
}

type testGetWeatherResponse struct {
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
func getTestWeather(lat, long float64) (string, error) {
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

	var response testGetWeatherResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}

	return fmt.Sprintf("The weather in (%.2f, %.2f) is %.2f°C", lat, long, response.CurrentWeather.Temperature), nil
}

func TestMain(m *testing.M) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if apiKey == "" || baseURL == "" {
		fmt.Println("OPENAI_API_KEY and OPENAI_BASE_URL are required")
		os.Exit(1)
	}

	var err error
	testOpenAi, err = New(Config{
		ApiKey:  apiKey,
		BaseURL: baseURL,
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	m.Run()
}

func TestChatCompletionSingleRound(t *testing.T) {
	messages := []param.Message{
		param.NewSystemMessage("You are a helpful assistant."),
		param.NewUserMessage(
			"Please get the weather in Shanghai, China, use approximate latitude and longitude, and return the weather in the format of 'The weather in xx is yy°C'"),
	}

	tools := []param.Tool{
		param.NewTool[testGetWeatherInput]("get_weather", "Get the weather for a given location"),
	}

	ctx := t.Context()
	defaultModel := "kimi-k2.5"

	resp, err := testOpenAi.ChatCompletion(ctx, &schema.Request{
		Model:       defaultModel,
		Messages:    messages,
		Temperature: 1,
		Tools:       tools,
		Thinking:    schema.EnableThinking(),
	})
	if err != nil {
		t.Fatalf("Failed to chat completion: %v", err)
	}

	output, _ := json.MarshalIndent(resp, "", " ")
	println(string(output))
}

func TestChatCompletion(t *testing.T) {
	messages := []param.Message{
		param.NewSystemMessage("You are a helpful assistant."),
		param.NewUserMessage(
			"Please get the weather in Shanghai, China, use approximate latitude and longitude, and return the weather in the format of 'The weather in xx is yy°C'"),
	}

	tools := []param.Tool{
		param.NewTool[testGetWeatherInput]("get_weather", "Get the weather for a given location"),
	}

	ctx := t.Context()
	defaultModel := "kimi-k2.5"

	maxRounds := 10

	usage := schema.CompletionUsage{}

	for round := range maxRounds {
		resp, err := testOpenAi.ChatCompletion(ctx, &schema.Request{
			Model:       defaultModel,
			Messages:    messages,
			Temperature: 1.0,
			Tools:       tools,
			Thinking:    schema.EnableThinking(),
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
		fmt.Printf("Reasoning: %s\n", choice.Message.ReasoningContent)

		if choice.Message.HasToolCalls() {
			fmt.Println("Tool Calls:")
			var reasoningContent *param.ReasoningContent
			if choice.Message.ReasoningContent != nil {
				reasoningContent = &param.ReasoningContent{Content: choice.Message.ReasoningContent.Content}
			}
			messages = append(messages,
				param.NewAssistantMessage(
					choice.Message.Content,
					choice.Message.GetToolCalls(), reasoningContent))

			for _, toolCall := range choice.Message.ToolCalls {
				fmt.Println(toolCall.Id)
				fmt.Println(toolCall.Function.Name)
				fmt.Println(toolCall.Function.Arguments)

				// add assistant message to messages
				// simulate append tool result to messages
				var input testGetWeatherInput
				err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input)
				if err != nil {
					messages = append(messages, param.NewToolMessage(toolCall.Id, fmt.Sprintf("Failed to unmarshal arguments: %v", err)))
				} else {
					result, err := getTestWeather(input.Lat, input.Long)
					if err != nil {
						messages = append(messages, param.NewToolMessage(toolCall.Id, fmt.Sprintf("Failed to get weather: %v", err)))
					} else {
						messages = append(messages, param.NewToolMessage(toolCall.Id, result))
					}
				}
			}
		} else {
			fmt.Println("No tool calls")
			break
		}
	}

	fmt.Printf("Total usage: %+v\n", usage)
}

func TestChatCompletionStream(t *testing.T) {
	messages := []param.Message{
		param.NewSystemMessage("You are a helpful assistant."),
		param.NewUserMessage("Tell me why the sky is blue."),
	}

	req := schema.NewRequest("kimi-k2-0905-preview", messages)
	req.Temperature = 0.8

	stream := testOpenAi.ChatCompletionStream(t.Context(), req)
	for chunk := range stream {
		if chunk.Err != nil {
			t.Fatalf("Failed to read stream: %v", chunk.Err)
		}

		fmt.Printf("Chunk: %+v\n", chunk.FirstChoice().Delta.Content)
	}
}

type testGetRandomListInput struct {
	N int `json:"n" jsonschema:"description=The number of random numbers to generate"`
}

type testGetRandomNameInput struct {
	Name string `json:"name" jsonschema:"description=random name"`
}

func TestChatCompletionStreamWithTools(t *testing.T) {
	messages := []param.Message{
		param.NewSystemMessage("You are a helpful assistant."),
		param.NewUserMessage("Return a random number and a random name using tools. YOU MUST USE TOOLS" +
			"You generate the parameters for the tools. And explain why in short, less then 50 words"),
	}

	tools := []param.Tool{
		param.NewTool[testGetRandomListInput]("get_random_list", "Return a random number."),
		param.NewTool[testGetRandomNameInput]("get_random_name", "Return a random name."),
	}

	req := schema.NewRequest("kimi-k2-0905-preview", messages)
	req.Tools = tools
	req.N = 2
	req.Temperature = 0.8
	stream := testOpenAi.ChatCompletionStream(t.Context(), req)

	choices, err := schema.SyncReadStream(stream)
	if err != nil {
		t.Fatalf("Failed to sync wait stream response: %v", err)
	}

	for _, choice := range choices {
		fmt.Printf("Choice[%d]: %+v\n", choice.Index, choice)
	}
}

func TestChatCompletionStreamWithToolsHandler(t *testing.T) {
	messages := []param.Message{
		param.NewSystemMessage("You are a helpful assistant."),
		param.NewUserMessage("Return a random number and a random name using tools. YOU MUST USE TOOLS" +
			"You generate the parameters for the tools. And explain why in detail"),
	}

	tools := []param.Tool{
		param.NewTool[testGetRandomListInput]("get_random_list", "Return a random number."),
		param.NewTool[testGetRandomNameInput]("get_random_name", "Return a random name."),
	}

	req := schema.NewRequest("kimi-k2-0905-preview", messages)
	req.Tools = tools
	req.Temperature = 0.8

	stream := testOpenAi.ChatCompletionStream(t.Context(), req)

	wg := sync.WaitGroup{}
	contentChCollection := schema.StreamResponseHandler(
		t.Context(),
		stream,
		func(ctx context.Context, tc schema.StreamChoiceDeltaToolCall) { // will be called in a new goroutine
			fmt.Printf("Tool Call: %+v\n", tc)
		})
	contentCh := contentChCollection.Content
	toolCallCh := contentChCollection.ToolCall

	wg.Add(1)
	go func() {
		defer wg.Done()
		for content := range contentCh {
			fmt.Printf("Content: %s\n", content.Content)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for tc := range toolCallCh {
			fmt.Printf("ToolCall: %+v\n", tc)
		}
	}()

	wg.Wait()
}
