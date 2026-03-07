package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ryanreadbooks/tokkibot/agent/tools/description"
	"github.com/ryanreadbooks/tokkibot/component/tool"
	"github.com/ryanreadbooks/tokkibot/config"

	"github.com/bmatcuk/doublestar/v4"
)

type GlobInput struct {
	Pattern   string `json:"pattern"   jsonschema:"description=The glob pattern to match files against"`
	Directory string `json:"directory" jsonschema:"description=The directory to search in. If not specified, the current directory will be used. IMPORTANT: Omit this field to use the default directory. DO NOT enter \"undefined\" or \"null\" - simply omit it for the default behavior. Must be a valid directory path if provided."`
}

func doGlobInvoke(ctx context.Context, meta tool.InvokeMeta, input *GlobInput) (string, error) {
	dir := config.GetProjectDir()
	if input.Directory != "" {
		var dir2 string
		dir2, err := filepath.Abs(input.Directory)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path of directory: %w", err)
		}

		dir = dir2
	}

	fs := os.DirFS(dir)
	matches, err := doublestar.Glob(fs,
		filepath.ToSlash(input.Pattern),
		doublestar.WithFailOnIOErrors(),
		doublestar.WithCaseInsensitive(),
		doublestar.WithNoHidden())
	if err != nil {
		return "", fmt.Errorf("failed to glob files: %w", err)
	}

	return strings.Join(matches, "\n"), nil
}

func Glob() tool.Invoker {
	info := tool.Info{
		Name:        ToolNameGlob,
		Description: description.GlobDescription,
	}

	return tool.NewInvoker(info, doGlobInvoke)
}
