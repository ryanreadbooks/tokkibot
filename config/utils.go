package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	homeDir     string
	homeDirOnce sync.Once
)

func Init() {
	GetConfigDir()
}

const configFileName = "config.yaml"

func GetConfigDir() string {
	homeDirOnce.Do(func() {
		home, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}
		homeDir = filepath.Join(home, ".tokkibot")
	})

	return homeDir
}

func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	return filepath.Join(home, ".tokkibot", configFileName), nil
}
