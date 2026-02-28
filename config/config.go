package config

import (
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

var (
	conf Config
	once sync.Once
)

type ProviderConfig struct {
	ApiKey       string `yaml:"api_key"`
	BaseURL      string `yaml:"base_url"`
	DefaultModel string `yaml:"default_model"`
}

// The configuration for the tokkibot.
type Config struct {
	DefaultProvider string                    `yaml:"default_provider"`
	Providers       map[string]ProviderConfig `yaml:"providers"`
	Adapters        AdapterConfig             `yaml:"adapters"`
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

	return
}

func GetConfig() Config {
	return conf
}
