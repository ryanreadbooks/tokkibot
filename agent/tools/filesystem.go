package tools

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ryanreadbooks/tokkibot/agent/ref"
	"github.com/ryanreadbooks/tokkibot/component/tool"
	"github.com/ryanreadbooks/tokkibot/config"
)

func resolvePath(path string, allowDirs []string) (string, error) {
	if filepath.IsAbs(path) {
		cleanPath := filepath.Clean(path)
		if len(allowDirs) > 0 {
			for _, allowDir := range allowDirs {
				if strings.HasPrefix(cleanPath, allowDir) {
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

type ReadFileInput struct {
	Path   string `json:"path"             jsonschema:"description=The path to the file read from"`
	Offset int    `json:"offset,omitempty" jsonschema:"description=Starting line number (1-indexed). Use for large files to read from specific line"`
	Limit  int    `json:"limit,omitempty"  jsonschema:"description=Number of lines to read. Use with offset for large files to read in chunks"`
}

// Tool to read a file contents.
//
// We restrict the directory to read from to avoid security issues.
func ReadFile(allowDirs []string) tool.Invoker {
	return tool.NewInvoker(tool.Info{
		Name: "read_file",
		Description: "Read the contents of a file at the given path. Output always always include numbers " +
			"in format 'LINE_NUMBER|LINE_CONTENT' (1-indexed). Supports reading partial content " +
			"by specifying line offset and limit for large files. ",
	}, func(ctx context.Context, input *ReadFileInput) (content string, err error) {
		// now we can read the file
		cleanPath, err := resolvePath(input.Path, allowDirs)
		if err != nil {
			return "", err
		}

		f, err := os.Open(cleanPath)
		if err != nil {
			return "", fmt.Errorf("failed to open file %s: %w", cleanPath, err)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lines := []string{}
		for scanner.Scan() {
			line := scanner.Text()
			lines = append(lines, line)
		}
		if err := scanner.Err(); err != nil {
			return "", err
		}

		start := 0
		if input.Offset != 0 {
			start = input.Offset - 1
		}
		start = max(start, 0)
		end := len(lines)
		if input.Limit != 0 {
			end = start + input.Limit
		}
		end = min(end, len(lines))

		selected := lines[start:end]
		numberedLines := []string{}
		for i := range selected {
			numberedLines = append(numberedLines, fmt.Sprintf("%d|%s", start+i+1, strings.TrimRight(selected[i], "\n")))
		}

		return strings.Join(numberedLines, "\n"), nil
	})
}

type WriteFileInput struct {
	Path    string `json:"path"    jsonschema:"description=The path to the file to write to"`
	Content string `json:"content" jsonschema:"description=The content to write to the file"`
}

// WriteFile tool to write content to a file at the given path.
func WriteFile(allowDirs []string) tool.Invoker {
	return tool.NewInvoker(tool.Info{
		Name:        "write_file",
		Description: "Write content to a file at the given path. Creates parent directories if necessary.",
	}, func(ctx context.Context, input *WriteFileInput) (result string, err error) {
		cleanPath, err := resolvePath(input.Path, allowDirs)
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

func ListDir(allowDirs []string) tool.Invoker {
	return tool.NewInvoker(tool.Info{
		Name:        "list_dir",
		Description: "List the contents of a directory.",
	}, func(ctx context.Context, input *ListDirInput) (result string, err error) {
		cleanPath, err := resolvePath(input.Path, allowDirs)
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

type EditFileInput struct {
	FileName   string `json:"file_name"             jsonschema:"description=The name of the file to edit"`
	NewString  string `json:"new_string"            jsonschema:"description=The new string to replace the old string with"`
	OldString  string `json:"old_string"            jsonschema:"description=The old string to replace"`
	ReplaceAll bool   `json:"replace_all,omitempty" jsonschema:"description=Whether to replace all occurrences of the old string or just the first one"`
}

func EditFile(allowDirs []string) tool.Invoker {
	return tool.NewInvoker(tool.Info{
		Name:        "edit_file",
		Description: "Edit the contents of a file at the given path by replacing the old string with the new string.",
	}, func(ctx context.Context, input *EditFileInput) (result string, err error) {
		cleanPath, err := resolvePath(input.FileName, allowDirs)
		if err != nil {
			return "", err
		}

		f, err := os.OpenFile(cleanPath, os.O_RDWR, 0664)
		if err != nil {
			return "", fmt.Errorf("failed to open file %s: %w", cleanPath, err)
		}
		defer f.Close()

		content, err := io.ReadAll(f)
		if err != nil {
			return "", fmt.Errorf("failed to read file %s: %w", cleanPath, err)
		}

	contentStr := string(content)
	if input.ReplaceAll {
		contentStr = strings.ReplaceAll(contentStr, input.OldString, input.NewString)
	} else {
		contentStr = strings.Replace(contentStr, input.OldString, input.NewString, 1)
	}

	// Seek to beginning and truncate file before writing
	if _, err = f.Seek(0, 0); err != nil {
		return "", fmt.Errorf("failed to seek file %s: %w", cleanPath, err)
	}
	if err = f.Truncate(0); err != nil {
		return "", fmt.Errorf("failed to truncate file %s: %w", cleanPath, err)
	}

	_, err = f.WriteString(contentStr)
	if err != nil {
		return "", fmt.Errorf("failed to write file %s: %w", cleanPath, err)
	}

	return fmt.Sprintf("File %s edited successfully", cleanPath), nil
	})
}

type LoadRefInput struct {
	Name string `json:"name" jsonschema:"description=The reference name to load"`
}

// Load ref, similar to read file, but for better understanding this is tool is seperated.
func LoadRef() tool.Invoker {
	return tool.NewInvoker(tool.Info{
		Name:        "load_ref",
		Description: "Load content from a previously stored reference (e.g., tool call results).",
	}, func(ctx context.Context, input *LoadRefInput) (string, error) {
		// refName example: @refs/xxx/xxx
		fullpath, err := ref.Fullpath(input.Name)
		if err != nil {
			return "", fmt.Errorf("invalid refname %s: %w", input, err)
		}

		content, err := os.ReadFile(fullpath)
		if err != nil {
			return "", fmt.Errorf("failed to read %s: %w", input, err)
		}

		return string(content), err
	})
}
