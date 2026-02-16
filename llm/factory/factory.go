package factory

import (
	"fmt"
	"os"

	"github.com/ryanreadbooks/tokkibot/llm"
	"github.com/ryanreadbooks/tokkibot/llm/openai"
)

const (
	DefaultAPIKeyEnv  = "TOKKIBOT_LLM_API_KEY"
	DefaultBaseURLEnv = "TOKKIBOT_LLM_BASE_URL"
)

type Style string

const (
	StyleOpenAI Style = "openai"
)

func DefaultOption() option {
	return option{
		style:   StyleOpenAI,
		apiKey:  os.Getenv(DefaultAPIKeyEnv),
		baseURL: os.Getenv(DefaultBaseURLEnv),
	}
}

type option struct {
	apiKey  string
	baseURL string

	style Style
}

func (o *option) apply(opts ...Option) {
	for _, opt := range opts {
		opt(o)
	}
}

type Option func(*option)

func WithStyle(s Style) Option {
	return func(o *option) {
		o.style = s
	}
}

func WithAPIKey(apiKey string) Option {
	return func(o *option) {
		o.apiKey = apiKey
	}
}

func WithBaseURL(baseURL string) Option {
	return func(o *option) {
		o.baseURL = baseURL
	}
}

func NewLLM(opts ...Option) (llm.LLM, error) {
	proOpt := DefaultOption()
	proOpt.apply(opts...)

	if proOpt.style == "" {
		return nil, fmt.Errorf("style is required")
	}

	switch proOpt.style {
	case StyleOpenAI:
		return openai.New(openai.Config{
			ApiKey:  proOpt.apiKey,
			BaseURL: proOpt.baseURL,
		})
	default:
		return nil, fmt.Errorf("unsupported style: %s", proOpt.style)
	}
}
