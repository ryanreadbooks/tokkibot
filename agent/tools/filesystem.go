package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ryanreadbooks/tokkibot/component/tool"
)

func resolvePath(path, allowDir string) (string, error) {
	cleanPath := filepath.Clean(path)
	if allowDir != "" {
		if !strings.HasPrefix(cleanPath, allowDir) {
			return "", fmt.Errorf("Path %s is outside of allowed directory %s", path, allowDir)
		}
	}

	return cleanPath, nil
}

type ReadFileInput struct {
	Path string `json:"path" jsonschema:"description=The path to the file to read from"`
}

// Tool to read a file contents.
//
// We restrict the directory to read from to avoid security issues.
func ReadFile(allowDir string) tool.Invoker {
	if allowDir != "" {
		allowDir = filepath.Clean(allowDir)
	}

	return tool.NewInvoker(tool.Info{
		Name:        "read_file",
		Description: "Read the contents of a file at the given path",
	}, func(ctx context.Context, input *ReadFileInput) (content string, err error) {
		// now we can read the file
		cleanPath, err := resolvePath(input.Path, allowDir)
		if err != nil {
			return "", err
		}

		fileContent, err := os.ReadFile(cleanPath)
		if err != nil {
			return "", fmt.Errorf("failed to read file %s: %w", cleanPath, err)
		}

		return string(fileContent), nil
	})
}

type WriteFileInput struct {
	Path    string `json:"path"    jsonschema:"description=The path to the file to write to"`
	Content string `json:"content" jsonschema:"description=The content to write to the file"`
}

// WriteFile tool to write content to a file at the given path.
func WriteFile(allowDir string) tool.Invoker {
	if allowDir != "" {
		allowDir = filepath.Clean(allowDir)
	}

	return tool.NewInvoker(tool.Info{
		Name:        "write_file",
		Description: "Write content to a file at the given path. Creates parent directories if necessary.",
	}, func(ctx context.Context, input *WriteFileInput) (result string, err error) {
		cleanPath, err := resolvePath(input.Path, allowDir)
		if err != nil {
			return "", err
		}

		err = os.MkdirAll(filepath.Dir(cleanPath), 0755)
		if err != nil {
			return "", fmt.Errorf("failed to create parent directories for %s: %w", input.Path, err)
		}

		// write the file
		err = os.WriteFile(cleanPath, []byte(input.Content), 0644)
		if err != nil {
			return "", fmt.Errorf("failed to write file %s: %w", cleanPath, err)
		}

		return fmt.Sprintf("File %s written successfully", cleanPath), nil
	})
}

type ListDirInput struct {
	Path string `json:"path" jsonschema:"description=The path to the directory to list"`
}

func ListDir(allowDir string) tool.Invoker {
	if allowDir != "" {
		allowDir = filepath.Clean(allowDir)
	}

	return tool.NewInvoker(tool.Info{
		Name:        "list_dir",
		Description: "List the contents of a directory.",
	}, func(ctx context.Context, input *ListDirInput) (result string, err error) {
		cleanPath, err := resolvePath(input.Path, allowDir)
		if err != nil {
			return "", err
		}

		entries, err := os.ReadDir(cleanPath)
		if err != nil {
			return "", fmt.Errorf("failed to read directory %s: %w", cleanPath, err)
		}

		if len(entries) == 0 {
			return "Directory is empty", nil
		}

		var ss strings.Builder
		ss.Grow(len(entries) * 16)
		for _, entry := range entries {
			if entry.IsDir() {
				ss.WriteString(fmt.Sprintf("📁%s", entry.Name()))
			} else {
				ss.WriteString(fmt.Sprintf("📄%s", entry.Name()))
			}
		}

		return ss.String(), nil
	})
}
