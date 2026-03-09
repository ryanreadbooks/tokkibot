package guard

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ryanreadbooks/tokkibot/config"
)

func ResolvePath(path string, allowDirs []string) (string, error) {
	if filepath.IsAbs(path) {
		cleanPath := filepath.Clean(path)
		if len(allowDirs) > 0 {
			for _, allowDir := range allowDirs {
				if isSubDir(allowDir, cleanPath) {
					return cleanPath, nil
				}
			}

			return "", fmt.Errorf("Path %s is outside of allowed directories %v", path, allowDirs)
		}

		return cleanPath, nil
	}

	// relative path
	return filepath.Join(config.GetProjectDir(), filepath.Clean(path)), nil
}

var denyWritePaths = []string{
	filepath.Join(config.GetHomeDir(), "refs"),
	filepath.Join(config.GetHomeDir(), "medias"),
	filepath.Join(config.GetHomeDir(), "sessions"),
	filepath.Join(config.GetHomeDir(), "crons"),
}

func isSubDir(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// self protection
func IsPathWriteProtected(path string) bool {
	var abs string
	if filepath.IsAbs(path) {
		abs = filepath.Clean(path)
	} else {
		abs = filepath.Join(config.GetProjectDir(), filepath.Clean(path))
	}

	for _, deny := range denyWritePaths {
		if isSubDir(deny, abs) {
			return true
		}
	}
	return false
}
