package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	workspaceDir     string
	workspaceDirOnce sync.Once

	projectDir     string
	projectDirOnce sync.Once
)

func MustInit() {
	GetWorkspaceDir()
	GetProjectDir()
	var err error
	conf, err = LoadConfig()
	if err != nil {
		panic(err)
	}
}

const configFileName = "config.yaml"

func GetWorkspaceDir() string {
	workspaceDirOnce.Do(func() {
		home, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}
		workspaceDir = filepath.Join(home, ".tokkibot")
	})

	return workspaceDir
}

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
