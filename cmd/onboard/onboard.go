package onboard

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ryanreadbooks/tokkibot/config"
	"gopkg.in/yaml.v3"

	"github.com/spf13/cobra"
)

var OnboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Initialize tokkibot configuration.",
	Long:  "Initialize tokkibot configuration.",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := runOnboard(args)
		if err != nil {
			// LOGGING
		}

		return nil
	},
}

func runOnboard(_ []string) error {
	configPath, err := config.GetConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	dir := filepath.Dir(configPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create config directory at %s: %w", dir, err)
		}
	}

	// check file exists, ask user if they want to overwrite
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		// ask user if they want to overwrite
		fmt.Printf("Config file already exists at %s, do you want to overwrite it? (y/n): ", configPath)
		var overwrite string
		fmt.Scanln(&overwrite)
		if overwrite != "y" {
			return nil
		}
	}

	cfg := config.BootstrapConfig()
	output, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	err = os.WriteFile(configPath, output, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Printf("Configuration written to %s\n", configPath)

	// prompts file init
	promptsPath := filepath.Join(dir, "prompts")
	if _, err := os.Stat(promptsPath); os.IsNotExist(err) {
		err = os.MkdirAll(promptsPath, 0755)
		if err != nil {
			return fmt.Errorf("failed to create prompts directory at %s: %w", promptsPath, err)
		}
	}

	promptsFiles := []string{
		"AGENTS.md",
	}

	for _, promptFile := range promptsFiles {
		promptContent, err := os.ReadFile(filepath.Join("workspace", "prompts", promptFile))
		if err != nil {
			return fmt.Errorf("failed to read prompt file %s: %w", promptFile, err)
		}

		err = os.WriteFile(filepath.Join(promptsPath, promptFile), promptContent, 0644)
		if err != nil {
			return fmt.Errorf("failed to write prompt file %s: %w", promptFile, err)
		}
	}

	fmt.Printf("Prompts written to %s\n", promptsPath)

	return nil
}
