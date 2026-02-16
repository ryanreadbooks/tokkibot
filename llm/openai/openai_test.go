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

	"github.com/ryanreadbooks/tokkibot/llm"
	"github.com/ryanreadbooks/tokkibot/llm/model"
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
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("The weather in %f, %f is %f°C", lat, long, response.CurrentWeather.Temperature), nil
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
		fmt.Printf("Failed to create LLM: %v\n", err)
		os.Exit(1)
	}
	m.Run()
}

func TestChatCompletionSingleRound(t *testing.T) {
	messages := []model.MessageParam{
		model.NewSystemMessageParam("You are a helpful assistant."),
		model.NewUserMessageParam(
			"Please get the weather in Shanghai, China, use approximate latitude and longitude, and return the weather in the format of 'The weather in xx is yy°C'"),
	}

	tools := []model.ToolParam{
		model.NewToolParam[testGetWeatherInput]("get_weather", "Get the weather for a given location"),
	}

	ctx := t.Context()
	defaultModel := "kimi-k2.5"

	resp, err := testOpenAi.ChatCompletion(ctx, &llm.Request{
		Model:       defaultModel,
		Messages:    messages,
		Temperature: 1,
		Tools:       tools,
		Thinking:    model.EnableThinking(),
	})
	if err != nil {
		t.Fatalf("Failed to chat completion: %v", err)
	}

	output, _ := json.MarshalIndent(resp, "", " ")
	println(string(output))
}

func TestChatCompletion(t *testing.T) {
	messages := []model.MessageParam{
		model.NewSystemMessageParam("You are a helpful assistant."),
		model.NewUserMessageParam(
			"Please get the weather in Shanghai, China, use approximate latitude and longitude, and return the weather in the format of 'The weather in xx is yy°C'"),
	}

	tools := []model.ToolParam{
		model.NewToolParam[testGetWeatherInput]("get_weather", "Get the weather for a given location"),
	}

	ctx := t.Context()
	defaultModel := "kimi-k2.5"

	maxRounds := 10

	usage := model.CompletionUsage{}

	for round := range maxRounds {
		resp, err := testOpenAi.ChatCompletion(ctx, &llm.Request{
			Model:       defaultModel,
			Messages:    messages,
			Temperature: 1.0,
			Tools:       tools,
			Thinking:    model.EnableThinking(),
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
			var reasoningContent *model.StringParam
			if choice.Message.ReasoningContent != "" {
				reasoningContent = &model.StringParam{Value: choice.Message.ReasoningContent}
			}
			messages = append(messages,
				model.NewAssistantMessageParam(
					choice.Message.Content,
					choice.Message.GetToolCallParams(), reasoningContent))

			for _, toolCall := range choice.Message.ToolCalls {
				fmt.Println(toolCall.Id)
				fmt.Println(toolCall.Function.Name)
				fmt.Println(toolCall.Function.Arguments)

				// add assistant message to messages
				// simulate append tool result to messages
				var input testGetWeatherInput
				err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input)
				if err != nil {
					messages = append(messages, model.NewToolMessageParam(toolCall.Id, fmt.Sprintf("Failed to unmarshal arguments: %v", err)))
				} else {
					result, err := getTestWeather(input.Lat, input.Long)
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

func TestChatCompletionStream(t *testing.T) {
	messages := []model.MessageParam{
		model.NewSystemMessageParam("You are a helpful assistant."),
		model.NewUserMessageParam("Give me a sentence about the weather in Shanghai, China."),
	}

	stream := testOpenAi.ChatCompletionStream(t.Context(), &llm.Request{
		Model:       "kimi-k2-0905-preview",
		Temperature: 1.0,
		Messages:    messages,
	})

	for chunk := range stream {
		if chunk.Err != nil {
			fmt.Printf("Error: %v\n", chunk.Err)
			break
		} else {
			fmt.Printf("Chunk: %s\n", chunk.FirstChoice().Delta.Content)
		}
	}
}

type testGetRandomListInput struct {
	Seed int `json:"seed" jsonschema:"description=random seed number"`
}

type testGetRandomNameInput struct {
	Name string `json:"name" jsonschema:"description=random name"`
}

func TestChatCompletionStreamWithTools(t *testing.T) {
	messages := []model.MessageParam{
		model.NewSystemMessageParam("You are a helpful assistant."),
		model.NewUserMessageParam("Return a random number and a random name using tools. YOU MUST USE TOOLS" +
			"You generate the parameters for the tools. And explain why in short, less then 50 words"),
	}

	tools := []model.ToolParam{
		model.NewToolParam[testGetRandomListInput]("get_random_list", "Return a random number."),
		model.NewToolParam[testGetRandomNameInput]("get_random_name", "Return a random name."),
	}

	req := llm.NewRequest("kimi-k2-0905-preview", messages)
	req.Tools = tools
	req.N = 2
	req.Temperature = 0.8
	stream := testOpenAi.ChatCompletionStream(t.Context(), req)

	choices, err := llm.SyncReadStream(stream)
	if err != nil {
		t.Fatalf("Failed to sync wait stream response: %v", err)
	}

	for _, choice := range choices {
		fmt.Printf("Choice[%d]: %+v\n", choice.Index, choice)
	}
}

func TestChatCompletionStreamWithToolsHandler(t *testing.T) {
	messages := []model.MessageParam{
		model.NewSystemMessageParam("You are a helpful assistant."),
		model.NewUserMessageParam("Return a random number and a random name using tools. YOU MUST USE TOOLS" +
			"You generate the parameters for the tools. And explain why in detail"),
	}

	tools := []model.ToolParam{
		model.NewToolParam[testGetRandomListInput]("get_random_list", "Return a random number."),
		model.NewToolParam[testGetRandomNameInput]("get_random_name", "Return a random name."),
	}

	req := llm.NewRequest("kimi-k2-0905-preview", messages)
	req.Tools = tools
	req.Temperature = 0.8

	stream := testOpenAi.ChatCompletionStream(t.Context(), req)

	wg := sync.WaitGroup{}
	contentChCollection := llm.StreamResponseHandler(
		t.Context(),
		stream,
		func(ctx context.Context, tc model.StreamChoiceDeltaToolCall) { // will be called in a new goroutine
			fmt.Printf("Tool Call: %+v\n", tc)
		})
	contentCh := contentChCollection.Content
	toolCallCh := contentChCollection.ToolCall

	wg.Add(1)
	fmt.Println("Model is thinking...")
	go func() {
		defer wg.Done()
		for content := range contentCh {
			fmt.Print(content.Content)
			os.Stdout.Sync()
		}

		fmt.Println("\nDone thinking...")
	}()

	wg.Add(1)
	fmt.Println("Gather tools calls parameters...")
	init := make(map[string]struct{})
	go func() {
		defer wg.Done()
		for toolCall := range toolCallCh {
			if _, ok := init[toolCall.Name]; !ok {
				init[toolCall.Name] = struct{}{}
				fmt.Printf("%s:\n", toolCall.Name)
			} else {
				fmt.Printf("%s", toolCall.ArgumentFragment)
			}
		}
	}()

	wg.Wait()
	fmt.Println("All done")
}
