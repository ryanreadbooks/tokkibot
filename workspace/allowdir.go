package workspace

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/ryanreadbooks/tokkibot/config"
)

// define allowed readable and writable directories in workspace
var (
	allowReadDirs = []string{
		"memory",
		"refs",
		"medias",
	}

	allowWriteDirs = []string{
		"memory",
	}

	readOnce           sync.Once
	allowingReadPaths  = []string{}
	writeOnce          sync.Once
	allowingWritePaths = []string{}
)

func GetAllowedReadPaths() []string {
	readOnce.Do(func() {
		for _, allowDir := range allowReadDirs {
			fullPath := filepath.Join(config.GetWorkspaceDir(), allowDir)
			allowingReadPaths = append(allowingReadPaths, fullPath)
		}
	})
	return allowingReadPaths
}

func GetAllowedWritePaths() []string {
	writeOnce.Do(func() {
		for _, allowDir := range allowWriteDirs {
			fullPath := filepath.Join(config.GetWorkspaceDir(), allowDir)
			allowingWritePaths = append(allowingWritePaths, fullPath)
		}
	})
	return allowingWritePaths
}

func IsAllowedReadPath(dir string) bool {
	readOnce.Do(func() {
		for _, allowDir := range allowReadDirs {
			fullPath := filepath.Join(config.GetWorkspaceDir(), allowDir)
			allowingReadPaths = append(allowingReadPaths, fullPath)
		}
	})

	fullPath := filepath.Join(config.GetWorkspaceDir(), dir)
	for _, allowingReadPath := range allowingReadPaths {
		if strings.HasPrefix(fullPath, allowingReadPath) {
			return true
		}
	}
	return false
}

func IsAllowedWritePath(dir string) bool {
	writeOnce.Do(func() {
		for _, allowDir := range allowWriteDirs {
			fullPath := filepath.Join(config.GetWorkspaceDir(), allowDir)
			allowingWritePaths = append(allowingWritePaths, fullPath)
		}
	})

	fullPath := filepath.Join(config.GetWorkspaceDir(), dir)
	for _, allowingWritePath := range allowingWritePaths {
		if strings.HasPrefix(fullPath, allowingWritePath) {
			return true
		}
	}
	return false
}
