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

type Compatibility string

const (
	CompatibilityOpenAI Compatibility = "openai"
)

func DefaultOption() option {
	return option{
		compatibility: CompatibilityOpenAI,
		apiKey:        os.Getenv(DefaultAPIKeyEnv),
		baseURL:       os.Getenv(DefaultBaseURLEnv),
	}
}

type option struct {
	apiKey  string
	baseURL string

	compatibility Compatibility
}

func (o *option) apply(opts ...Option) {
	for _, opt := range opts {
		opt(o)
	}
}

type Option func(*option)

func WithCompatibility(compatibility Compatibility) Option {
	return func(o *option) {
		o.compatibility = compatibility
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

	if proOpt.compatibility == "" {
		return nil, fmt.Errorf("compatibility is required")
	}

	switch proOpt.compatibility {
	case CompatibilityOpenAI:
		return openai.New(openai.Config{
			ApiKey:  proOpt.apiKey,
			BaseURL: proOpt.baseURL,
		})
	default:
		return nil, fmt.Errorf("unsupported compatibility: %s", proOpt.compatibility)
	}
}
