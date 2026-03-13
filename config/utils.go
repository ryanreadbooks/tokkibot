package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/ryanreadbooks/tokkibot/pkg/log"
)

const (
	MainAgentName  = "main"
	CronsAgentName = "__cron"
)

const (
	sessionDirName = "sessions"
)

var (
	// tokkibot home directory, usually $HOME/.tokkibot
	homeDir     string
	homeDirOnce sync.Once

	// current working directory
	projectDir     string
	projectDirOnce sync.Once
)

func MustInit() {
	GetHomeDir()
	GetProjectDir()

	// Initialize logger first
	if err := log.Init(GetLogsDir()); err != nil {
		panic(fmt.Errorf("failed to init logger: %w", err))
	}

	var err error
	conf, err = LoadConfig()
	if err != nil {
		panic(err)
	}
}

// GetLogsDir returns the logs directory path: ~/.tokkibot/logs
func GetLogsDir() string {
	return filepath.Join(GetHomeDir(), "logs")
}

const configFileName = "config.json"

// GetHomeDir returns the tokkibot home directory, usually $HOME/.tokkibot.
// This is the root directory for all shared resources (config, refs, medias, crons, skills, sessions).
func GetHomeDir() string {
	homeDirOnce.Do(func() {
		home, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}
		homeDir = filepath.Join(home, ".tokkibot")
	})

	return homeDir
}

// GetWorkspaceDir is an alias for GetHomeDir.
// Deprecated: Use GetHomeDir instead.
func GetWorkspaceDir() string {
	return GetHomeDir()
}

// GetAgentWorkspaceDir returns the workspace directory for the specified agent.
//   - main agent: ~/.tokkibot/workspace
//   - other agents: ~/.tokkibot/workspace-{agentName}
func GetAgentWorkspaceDir(agentName string) string {
	if agentName == MainAgentName {
		return filepath.Join(GetHomeDir(), "workspace")
	}
	return filepath.Join(GetHomeDir(), "workspace-"+agentName)
}

// GetAgentSessionsDir returns the sessions directory for the specified agent.
// Path: ~/.tokkibot/sessions/{agentName}
func GetAgentSessionsDir(agentName string) string {
	return filepath.Join(GetAgentWorkspaceDir(agentName), sessionDirName)
}

func GetSubAgentSessionsDir(mainAgent, subAgent string) string {
	return filepath.Join(GetAgentWorkspaceDir(mainAgent), "subagents", subAgent, sessionDirName)
}

func GetCronSessionsDir() string {
	return filepath.Join(GetHomeDir(), sessionDirName)
}

// GetProjectDir returns the current working directory (project directory)
func GetProjectDir() string {
	projectDirOnce.Do(func() {
		cwd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		projectDir = cwd
	})

	return projectDir
}

func GetWorkspaceConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	return filepath.Join(home, ".tokkibot", configFileName), nil
}
