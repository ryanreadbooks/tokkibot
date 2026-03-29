package workspace

import (
	"path/filepath"

	"github.com/ryanreadbooks/tokkibot/config"
)

// Shared dirs under homeDir (~/.tokkibot/)
var sharedReadDirs = []string{
	"refs",
	"medias",
	"skills",
}

// Per-agent dirs under agentWorkspace (~/.tokkibot/workspace[-{name}]/)
var agentReadDirs = []string{
	"memory",
}

var agentWriteDirs = []string{
	"memory",
}

// GetAllowedReadPaths returns all allowed read paths for the given agent workspace.
// Includes shared dirs (refs, medias) from homeDir and agent-specific dirs (memory) from agentWorkspace.
func GetAllowedReadPaths(agentWorkspace string) []string {
	home := config.GetHomeDir()
	paths := make([]string, 0, len(sharedReadDirs)+len(agentReadDirs))
	for _, dir := range sharedReadDirs {
		paths = append(paths, filepath.Join(home, dir))
	}
	for _, dir := range agentReadDirs {
		paths = append(paths, filepath.Join(agentWorkspace, dir))
	}

	paths = append(paths, agentWorkspace)
	return paths
}

// GetAllowedWritePaths returns all allowed write paths for the given agent workspace.
func GetAllowedWritePaths(agentWorkspace string) []string {
	paths := make([]string, 0, len(agentWriteDirs))
	for _, dir := range agentWriteDirs {
		paths = append(paths, filepath.Join(agentWorkspace, dir))
	}
	paths = append(paths, agentWorkspace)
	
	return paths
}
