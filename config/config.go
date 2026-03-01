package config

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
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
	ApiKey                       string  `yaml:"api_key"`
	BaseURL                      string  `yaml:"base_url"`
	DefaultModel                 string  `yaml:"default_model"`
	EnableThinking               *bool   `yaml:"enable_thinking,omitempty"`
	Temperature                  float64 `yaml:"temperature,omitempty"`
	MaxTokens                    int     `yaml:"max_tokens,omitempty"`
	WindowLimit                  int64   `yaml:"window_limit,omitempty"`
	CompactThresholdPercentage   float64 `yaml:"compact_threshold_percentage,omitempty"`
	SummarizeThresholdPercentage float64 `yaml:"summarize_threshold_percentage,omitempty"`
	ToolCallCompressThreshold    int     `yaml:"tool_call_compress_threshold,omitempty"`
}

type AgentConfig struct {
	MaxIteration int `yaml:"max_iteration"`
}

// The configuration for the tokkibot.
type Config struct {
	DefaultProvider string                    `yaml:"default_provider"`
	Providers       map[string]ProviderConfig `yaml:"providers"`
	Adapters        AdapterConfig             `yaml:"adapters"`
	Agent           AgentConfig               `yaml:"agent"`
}

func (c *Config) ToYaml() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	encoder := yaml.NewEncoder(buf)
	encoder.SetIndent(4)
	err := encoder.Encode(c)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
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

	err = yaml.Unmarshal(content, &c)
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
