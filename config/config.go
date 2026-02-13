package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
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
}

func BootstrapConfig() Config {
	return Config{
		DefaultProvider: "openai",
		Providers: map[string]ProviderConfig{
			"openai": {
				ApiKey:       "",
				BaseURL:      "https://api.openai.com/v1",
				DefaultModel: "gpt-4o-mini",
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
