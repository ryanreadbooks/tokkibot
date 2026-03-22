package config

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
)

var conf Config

// Default values for provider config
const (
	defaultEnableThinking               = false
	defaultTemperature                  = -1 // use model default
	defaultMaxTokens                    = -1 // use model default
	defaultWindowLimit                  = 100000
	defaultCompactThresholdPercentage   = 0.70
	defaultSummarizeThresholdPercentage = 0.60
	defaultToolCallCompressThreshold    = 30
	defaultMaxIteration                 = 30
	defaultStyle                        = "openai"
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
	Style                        string  `json:"style,omitempty"`
}

type AgentBindingMatch struct {
	Channel string   `json:"channel"`
	Account string   `json:"account"`
	ChatIds []string `json:"chatIds,omitempty"` // optional: specific chatIds this agent handles; empty means all
}

type AgentBinding struct {
	Match AgentBindingMatch `json:"match"`
}

type SandboxConfig struct {
	Enabled        bool     `json:"enabled"`
	ReadOnlyPaths  []string `json:"readOnlyPaths,omitempty"`
	ReadWritePaths []string `json:"readWritePaths,omitempty"`
}

func (c *SandboxConfig) IsEnabled() bool {
	return c != nil && c.Enabled && runtime.GOOS == "linux"
}

func (c *SandboxConfig) GetReadOnlyPaths() []string {
	if c == nil {
		return nil
	}
	return c.ReadOnlyPaths
}

func (c *SandboxConfig) GetReadWritePaths() []string {
	if c == nil {
		return nil
	}
	return c.ReadWritePaths
}

type AgentEntry struct {
	Name         string                `json:"name"`
	MaxIteration int                   `json:"maxIteration"`
	Provider     string                `json:"provider"`
	Model        string                `json:"model,omitempty"`
	Binding      *AgentBinding         `json:"binding,omitempty"`
	Sandbox      *SandboxConfig        `json:"sandbox,omitempty"`
	Heartbeat    *AgentHeartbeatConfig `json:"heartbeat,omitempty"`
}

type ChannelEntry struct {
	Name    string                     `json:"name"`
	Account map[string]json.RawMessage `json:"account"`
}

type Config struct {
	Providers map[string]ProviderConfig `json:"providers"`
	Agents    []AgentEntry              `json:"agents"`
	Channels  []ChannelEntry            `json:"channels"`
}

func (c *Config) ToJson() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

type AgentHeartbeatConfig struct {
	Every  string `json:"every"`  // 30m
	Target string `json:"target"` // target channel
	To     string `json:"to"`     // chatid of target channel
	Prompt string `json:"prompt"` // prompt to send
}

func BootstrapConfig() Config {
	return Config{
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
			"deepseek": {
				ApiKey:       os.Getenv("DEEPSEEK_API_KEY"),
				BaseURL:      "https://api.deepseek.com/v1",
				DefaultModel: "deepseek-reasoner",
			},
		},
		Agents: []AgentEntry{
			{
				Name:         MainAgentName,
				MaxIteration: defaultMaxIteration,
				Provider:     "moonshot",
				Model:        "kimi-k2.5",
				Binding: &AgentBinding{
					Match: AgentBindingMatch{
						Channel: "lark",
						Account: "default",
					},
				},
			},
		},
		Channels: []ChannelEntry{
			{
				Name: "lark",
				Account: map[string]json.RawMessage{
					"default": json.RawMessage(`{"appId": "", "appSecret": ""}`),
				},
			},
		},
	}
}

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

func (ae *AgentEntry) applyDefaults(providers map[string]ProviderConfig) {
	if ae.MaxIteration == 0 {
		ae.MaxIteration = defaultMaxIteration
	}
	if ae.Model == "" {
		if p, ok := providers[ae.Provider]; ok {
			ae.Model = p.DefaultModel
		}
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

	for name, providerCfg := range c.Providers {
		providerCfg.applyDefaults()
		c.Providers[name] = providerCfg
	}

	for i := range c.Agents {
		c.Agents[i].applyDefaults(c.Providers)
	}

	return
}

func GetConfig() Config {
	return conf
}

// SaveConfig saves the current config to disk
func SaveConfig() error {
	configPath, err := GetWorkspaceConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	data, err := conf.ToJson()
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// UpdateAgentProviderAndModel updates the provider and model for an agent and saves to disk
func UpdateAgentProviderAndModel(agentName, provider, model string) error {
	for i := range conf.Agents {
		if conf.Agents[i].Name == agentName {
			conf.Agents[i].Provider = provider
			conf.Agents[i].Model = model
			return SaveConfig()
		}
	}
	return fmt.Errorf("agent not found: %s", agentName)
}

// GetAgentEntry returns the agent entry for the given agent id.
// Returns nil if not found.
func GetAgentEntry(name string) *AgentEntry {
	for i := range conf.Agents {
		if conf.Agents[i].Name == name {
			return &conf.Agents[i]
		}
	}
	return nil
}

// GetChannelEntry returns the channel entry for the given channel name.
// Returns nil if not found.
func GetChannelEntry(channelName string) *ChannelEntry {
	for i := range conf.Channels {
		if conf.Channels[i].Name == channelName {
			return &conf.Channels[i]
		}
	}
	return nil
}

// GetChannelAccountRaw returns raw JSON config for a channel account.
func GetChannelAccountRaw(channelName, accountName string) (json.RawMessage, bool) {
	ch := GetChannelEntry(channelName)
	if ch == nil {
		return nil, false
	}
	raw, ok := ch.Account[accountName]
	return raw, ok
}

func (pc ProviderConfig) HasThinkingSet() bool {
	return pc.EnableThinking != nil
}

// IsThinkingEnabled returns whether thinking is enabled for this provider
func (pc ProviderConfig) IsThinkingEnabled() bool {
	if pc.EnableThinking == nil {
		return defaultEnableThinking
	}
	return *pc.EnableThinking
}

func (pc ProviderConfig) GetContextCompactThreshold() int64 {
	return int64(float64(pc.WindowLimit) * pc.CompactThresholdPercentage)
}

func (pc ProviderConfig) GetContextSummarizeThreshold() int64 {
	return int64(float64(pc.WindowLimit) * pc.SummarizeThresholdPercentage)
}
