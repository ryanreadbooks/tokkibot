package onboard

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/workspace"

	"github.com/spf13/cobra"
)

var agentName string

var OnboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Initialize tokkibot configuration.",
	Long:  "Initialize tokkibot configuration and agent workspace.",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := runOnboard(args)
		if err != nil {
			return fmt.Errorf("failed to run onboard: %w", err)
		}

		return nil
	},
}

func init() {
	OnboardCmd.Flags().StringVar(&agentName, "agent", config.MainAgentName, "Agent name to onboard")
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
	output, err := cfg.ToJson()
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
	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		if err = os.MkdirAll(workspaceDir, 0755); err != nil {
			return fmt.Errorf("failed to create workspace directory at %s: %w", workspaceDir, err)
		}
	}

	// check if any prompt files already exist
	hasExisting := false
	promptFiles, err := workspace.PromptsFs.ReadDir("prompts")
	if err != nil {
		return fmt.Errorf("failed to read embedded prompt files: %w", err)
	}
	for _, pf := range promptFiles {
		if !pf.IsDir() {
			if _, err := os.Stat(filepath.Join(workspaceDir, pf.Name())); err == nil {
				hasExisting = true
				break
			}
		}
	}

	doInit := true
	if hasExisting {
		fmt.Printf("Prompt files already exist at %s, do you want to overwrite them? (y/n): ", workspaceDir)
		var overwrite string
		fmt.Scanln(&overwrite)
		if overwrite != "y" && overwrite != "Y" {
			doInit = false
		}
	}

	if !doInit {
		return nil
	}

	for _, promptFile := range promptFiles {
		if promptFile.IsDir() {
			continue
		}

		embeddedPath := filepath.Join("prompts", promptFile.Name())
		content, err := workspace.PromptsFs.ReadFile(embeddedPath)
		if err != nil {
			return fmt.Errorf("failed to read prompt file %s: %w", embeddedPath, err)
		}

		err = os.WriteFile(filepath.Join(workspaceDir, promptFile.Name()), content, 0644)
		if err != nil {
			return fmt.Errorf("failed to write prompt file %s: %w", promptFile.Name(), err)
		}
	}

	fmt.Printf("Prompt files written to %s\n", workspaceDir)

	return nil
}

func bootstrapMemory(workspaceDir string) error {
	targetMemoryPath := filepath.Join(workspaceDir, "memory")
	if _, err := os.Stat(targetMemoryPath); os.IsNotExist(err) {
		err = os.MkdirAll(targetMemoryPath, 0755)
		if err != nil {
			return fmt.Errorf("failed to create memory directory at %s: %w", targetMemoryPath, err)
		}
	}

	targetMemoryDir, err := os.ReadDir(targetMemoryPath)
	if err != nil {
		return fmt.Errorf("failed to read memory directory at %s: %w", targetMemoryPath, err)
	}

	doInit := true
	if len(targetMemoryDir) > 0 {
		fmt.Printf("Memory files already exist at %s, do you want to overwrite them? (y/n): ", targetMemoryPath)
		var overwrite string
		fmt.Scanln(&overwrite)
		if overwrite != "y" && overwrite != "Y" {
			doInit = false
		}
	}

	if !doInit {
		return nil
	}

	memoryFiles, err := workspace.MemoryFs.ReadDir("memory")
	if err != nil {
		return fmt.Errorf("failed to read memory files: %w", err)
	}

	for _, memoryFile := range memoryFiles {
		if memoryFile.IsDir() {
			continue
		}

		filePath := filepath.Join("memory", memoryFile.Name())
		content, err := workspace.MemoryFs.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read memory file %s: %w", filePath, err)
		}

		err = os.WriteFile(filepath.Join(targetMemoryPath, memoryFile.Name()), content, 0644)
		if err != nil {
			return fmt.Errorf("failed to write memory file %s: %w", filePath, err)
		}
	}

	fmt.Printf("Memory files written to %s\n", targetMemoryPath)

	return nil
}

func runOnboard(_ []string) error {
	homeDir := config.GetHomeDir()
	if _, err := os.Stat(homeDir); os.IsNotExist(err) {
		if err = os.MkdirAll(homeDir, 0755); err != nil {
			return fmt.Errorf("failed to create home directory at %s: %w", homeDir, err)
		}
	}

	// only bootstrap global config for main agent
	if agentName == config.MainAgentName {
		configPath, err := config.GetWorkspaceConfigPath()
		if err != nil {
			return fmt.Errorf("failed to get config path: %w", err)
		}
		if err := bootstrapConfig(configPath); err != nil {
			return fmt.Errorf("failed to bootstrap config: %w", err)
		}
	}

	agentWorkspace := config.GetAgentWorkspaceDir(agentName)
	fmt.Printf("Initializing agent %s workspace at %s\n", agentName, agentWorkspace)

	if err := bootstrapPrompts(agentWorkspace); err != nil {
		return fmt.Errorf("failed to bootstrap prompts: %w", err)
	}

	if err := bootstrapMemory(agentWorkspace); err != nil {
		return fmt.Errorf("failed to bootstrap memory: %w", err)
	}

	return nil
}
