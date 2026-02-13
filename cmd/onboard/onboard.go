package onboard

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/workspace"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var OnboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Initialize tokkibot configuration.",
	Long:  "Initialize tokkibot configuration.",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := runOnboard(args)
		if err != nil {
			return fmt.Errorf("failed to run onboard: %w", err)
		}

		return nil
	},
}

func bootstrapConfig(configPath string) error {
	// check file exists, ask user if they want to overwrite
	doInit := true
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		// ask user if they want to overwrite
		fmt.Printf("Config file already exists at %s, do you want to overwrite it? (y/n): ", configPath)
		var overwrite string
		fmt.Scanln(&overwrite)
		if overwrite != "y" && overwrite != "Y" {
			doInit = false
		}
	}

	if !doInit {
		return nil
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

	return nil
}

func bootstrapPrompts(workspaceDir string) error {
	// prompts file init
	targetPromptPath := filepath.Join(workspaceDir, "prompts")
	if _, err := os.Stat(targetPromptPath); os.IsNotExist(err) {
		err = os.MkdirAll(targetPromptPath, 0755)
		if err != nil {
			return fmt.Errorf("failed to create prompts directory at %s: %w", targetPromptPath, err)
		}
	}
	// first check prompt folder exists and not empty
	targetPromptDir, err := os.ReadDir(targetPromptPath)
	if err != nil {
		return fmt.Errorf("failed to read prompt directory at %s: %w", targetPromptPath, err)
	}

	doInit := true
	if len(targetPromptDir) > 0 {
		fmt.Printf("Prompt files already exist at %s, do you want to overwrite them? (y/n): ", targetPromptPath)
		var overwrite string
		fmt.Scanln(&overwrite)
		if overwrite != "y" && overwrite != "Y" {
			doInit = false
		}
	}

	if !doInit {
		return nil
	}

	promptFiles, err := workspace.PromptsFs.ReadDir("prompts")
	if err != nil {
		return fmt.Errorf("failed to read prompt files: %w", err)
	}

	for _, promptFile := range promptFiles {
		if promptFile.IsDir() {
			continue
		}

		// file
		filePath := filepath.Join("prompts", promptFile.Name())
		content, err := workspace.PromptsFs.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read prompt file %s: %w", filePath, err)
		}

		err = os.WriteFile(filepath.Join(targetPromptPath, promptFile.Name()), content, 0644)
		if err != nil {
			return fmt.Errorf("failed to write prompt file %s: %w", filePath, err)
		}
	}

	fmt.Printf("Prompt files written to %s\n", targetPromptPath)

	return nil
}

func runOnboard(_ []string) error {
	configPath, err := config.GetWorkspaceConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	workspaceDir := filepath.Dir(configPath)
	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		err = os.MkdirAll(workspaceDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create config directory at %s: %w", workspaceDir, err)
		}
	}

	if err := bootstrapConfig(configPath); err != nil {
		return fmt.Errorf("failed to bootstrap config: %w", err)
	}

	if err := bootstrapPrompts(workspaceDir); err != nil {
		return fmt.Errorf("failed to bootstrap prompts: %w", err)
	}

	return nil
}
