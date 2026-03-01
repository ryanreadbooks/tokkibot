package media

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ryanreadbooks/tokkibot/config"
)

const (
	mediaRefDir    = "medias"
	MediaRefPrefix = "@medias/"
)

// name: without @medias/ prefix
func realMediaRefFilename(name string) string {
	return filepath.Join(config.GetWorkspaceDir(), mediaRefDir, name)
}

func FullMediaPath(ref string) (string, error) {
	if !strings.HasPrefix(ref, MediaRefPrefix) {
		return "", fmt.Errorf("invalid ref name format")
	}

	name, ok := strings.CutPrefix(ref, MediaRefPrefix)
	if ok {
		return realMediaRefFilename(name), nil
	}

	return "", fmt.Errorf("ref %s not found", ref)
}

func SaveMedia(data []byte, key string) (string, error) {
	fullpath := realMediaRefFilename(key)
	err := os.MkdirAll(filepath.Dir(fullpath), 0755)
	if err != nil {
		return "", err
	}

	err = os.WriteFile(fullpath, data, 0644)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s%s", MediaRefPrefix, key), nil
}

// load media ref
// ref: @medias/xxx
func LoadMedia(ref string) ([]byte, error) {
	fullpath, err := FullMediaPath(ref)
	if err != nil {
		return nil, err
	}

	return os.ReadFile(fullpath)
}
