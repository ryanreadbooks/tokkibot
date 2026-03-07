package config

import (
	"encoding/json"
	"fmt"
	"os"
)

var conf Config

// Default values for provider config
const (
	defaultEnableThinking               = false
	defaultTemperature                  = -1
	defaultMaxTokens                    = 32768
	defaultWindowLimit                  = 100000
	defaultCompactThresholdPercentage   = 0.70
	defaultSummarizeThresholdPercentage = 0.60
	defaultToolCallCompressThreshold    = 30
)

type ProviderConfig struct {
	ApiKey                       string  `json:"apiKey"`
	BaseURL                      string  `json:"baseURL"`
	DefaultModel                 string  `json:"defaultModel"`
	EnableThinking               *bool   `json:"enableThinking,omitempty"`
	Temperature                  float64 `json:"temperature,omitempty"`
	MaxTokens                    int     `json:"maxTokens,omitempty"`
	WindowLimit                  int64   `json:"windowLimit,omitempty"`
	CompactThresholdPercentage   float64 `json:"compactThresholdPercentage,omitempty"`
	SummarizeThresholdPercentage float64 `json:"summarizeThresholdPercentage,omitempty"`
	ToolCallCompressThreshold    int     `json:"toolCallCompressThreshold,omitempty"`
}

type AgentConfig struct {
	MaxIteration int `json:"maxIteration"`
}

// The configuration for the tokkibot.
type Config struct {
	DefaultProvider string                    `json:"defaultProvider"`
	Providers       map[string]ProviderConfig `json:"providers"`
	Adapters        AdapterConfig             `json:"adapters"`
	Agent           AgentConfig               `json:"agent"`
}

func (c *Config) ToJson() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

func BootstrapConfig() Config {
	return Config{
		DefaultProvider: "moonshot",
		Providers: map[string]ProviderConfig{
			"openai": {
				ApiKey:       os.Getenv("OPENAI_API_KEY"),
				BaseURL:      "https://api.openai.com/v1",
				DefaultModel: "gpt-4o-mini",
			},
			"moonshot": {
				ApiKey:       os.Getenv("MOONSHOT_API_KEY"),
				BaseURL:      "https://api.moonshot.cn/v1",
				DefaultModel: "kimi-k2.5",
			},
		},
		Adapters: AdapterConfig{
			Lark: LarkAdapterConfig{
				AppId:     "",
				AppSecret: "",
			},
		},
		Agent: AgentConfig{
			MaxIteration: 30,
		},
	}
}

// applyProviderDefaults applies default values to provider config if not set
func (pc *ProviderConfig) applyDefaults() {
	if pc.EnableThinking == nil {
		enableThinking := defaultEnableThinking
		pc.EnableThinking = &enableThinking
	}
	if pc.Temperature == 0 {
		pc.Temperature = defaultTemperature
	}
	if pc.MaxTokens == 0 {
		pc.MaxTokens = defaultMaxTokens
	}
	if pc.WindowLimit == 0 {
		pc.WindowLimit = defaultWindowLimit
	}
	if pc.CompactThresholdPercentage == 0 {
		pc.CompactThresholdPercentage = defaultCompactThresholdPercentage
	}
	if pc.SummarizeThresholdPercentage == 0 {
		pc.SummarizeThresholdPercentage = defaultSummarizeThresholdPercentage
	}
	if pc.ToolCallCompressThreshold == 0 {
		pc.ToolCallCompressThreshold = defaultToolCallCompressThreshold
	}
}

func LoadConfig() (c Config, err error) {
	c = BootstrapConfig()
	configPath, err := GetWorkspaceConfigPath()
	if err != nil {
		err = fmt.Errorf("failed to get config path: %w", err)
		return
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		err = fmt.Errorf("failed to read config file: %w", err)
		return
	}

	err = json.Unmarshal(content, &c)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal config file: %w", err)
		return
	}

	// Apply defaults to all providers
	for name, providerCfg := range c.Providers {
		providerCfg.applyDefaults()
		c.Providers[name] = providerCfg
	}

	return
}

func GetConfig() Config {
	return conf
}

func GetAgentConfig() AgentConfig {
	return conf.Agent
}

// IsThinkingEnabled returns whether thinking is enabled for this provider
func (pc ProviderConfig) IsThinkingEnabled() bool {
	if pc.EnableThinking == nil {
		return defaultEnableThinking
	}
	return *pc.EnableThinking
}

// GetContextCompactThreshold returns the calculated compact threshold
func (pc ProviderConfig) GetContextCompactThreshold() int64 {
	return int64(float64(pc.WindowLimit) * pc.CompactThresholdPercentage)
}

// GetContextSummarizeThreshold returns the calculated summarize threshold
func (pc ProviderConfig) GetContextSummarizeThreshold() int64 {
	return int64(float64(pc.WindowLimit) * pc.SummarizeThresholdPercentage)
}
