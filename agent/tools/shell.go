package tools

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/ryanreadbooks/tokkibot/component/tool"
	"github.com/ryanreadbooks/tokkibot/pkg/bash"
	"github.com/ryanreadbooks/tokkibot/pkg/os"
)

const (
	maxAllowedShellOutputLen = 15000
	shellExecTimeout         = 60 * time.Second
)

type shellResultTag string

const (
	shellBlockedTag shellResultTag = "<shell_blocked>"
	shellRunErrTag  shellResultTag = "<shell_run_error>"
)

var errDangerousCommand = errors.New("dangerous command blocked")

func wrapShellError(err error, errTag shellResultTag) error {
	return fmt.Errorf("%s%w%s", errTag, err, errTag)
}

// shell command input
type ShellInput struct {
	Command    string `json:"command"               jsonschema:"description=The command to execute along with its arguments"`
	WorkingDir string `json:"working_dir,omitempty" jsonschema:"description=The working directory to execute the command in"`
}

// check if the shell command is safe to execute
func safeCheckShellCommand(command string) error {
	for _, p := range dangerousPatterns {
		if p.MatchString(command) {
			return errDangerousCommand
		}
	}
	return nil
}

// Tool to execute a command under the optional given working directory.
func Shell() tool.Invoker {
	return tool.NewInvoker(tool.Info{
		Name: "shell",
		Description: fmt.Sprintf(
			"Execute a shell command in %s under the optional given working directory, current working directory will be used if not provided.",
			os.GetSystemDistro(),
		),
	}, func(ctx context.Context, input *ShellInput) (string, error) {
		err := safeCheckShellCommand(input.Command)
		if err != nil {
			return "", wrapShellError(err, shellBlockedTag)
		}

		name, args := bash.ParseCommand(input.Command)
		if name == "" {
			return "", wrapShellError(errors.New("empty command"), shellBlockedTag)
		}

		ctx, cancel := context.WithTimeout(ctx, shellExecTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, name, args...)
		if input.WorkingDir != "" {
			cleanWd, _ := resolvePath(input.WorkingDir, []string{})
			cmd.Dir = cleanWd
		}

		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", wrapShellError(fmt.Errorf("%w: %s", err, string(output)), shellRunErrTag)
		}

		outputStr := string(output)

		// truncate output
		if len(outputStr) > maxAllowedShellOutputLen {
			more := len(output) - maxAllowedShellOutputLen
			outputStr = outputStr[:maxAllowedShellOutputLen] + fmt.Sprintf("\n... (truncated, %d more chars)", more)
		}

		return string(output), nil
	})
}
