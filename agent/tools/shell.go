package tools

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/ryanreadbooks/tokkibot/component/tool"
	"github.com/ryanreadbooks/tokkibot/pkg/bash"
	pkgos "github.com/ryanreadbooks/tokkibot/pkg/os"
)

const (
	maxAllowedShellOutputLen = 15000
	shellExecTimeout         = 60 * time.Second
)

type shellResultTag string

const (
	shellBlockedTag        shellResultTag = "<shell_blocked>"
	shellRunErrTag         shellResultTag = "<shell_run_error>"
	shellConfirmNeededTag  shellResultTag = "<shell_confirm_needed>"
)

var (
	errDangerousCommand = errors.New("dangerous command blocked")
	errConfirmNeeded    = errors.New("command requires user confirmation")
)

// ConfirmationRequiredError indicates a command needs user confirmation
type ConfirmationRequiredError struct {
	Command string
}

func wrapShellError(err error, errTag shellResultTag) error {
	return fmt.Errorf("%s%w%s", errTag, err, errTag)
}

// shell command input
type ShellInput struct {
	Command    string `json:"command"               jsonschema:"description=The command to execute along with its arguments"`
	WorkingDir string `json:"working_dir,omitempty" jsonschema:"description=The working directory to execute the command in"`
}

// checkCommandNeedsConfirmation checks if command requires user confirmation
func checkCommandNeedsConfirmation(command string) bool {
	for _, p := range confirmRequiredPatterns {
		if p.MatchString(command) {
			return true
		}
	}
	return false
}

// checkCommandBlocked checks if command is completely blocked
func checkCommandBlocked(command string) bool {
	for _, p := range dangerousPatterns {
		if p.MatchString(command) {
			return true
		}
	}
	return false
}

func (e *ConfirmationRequiredError) Error() string {
	return fmt.Sprintf("Command '%s' requires user confirmation. Please confirm to proceed.", e.Command)
}

// ShellConfirmer is an interface for confirming shell commands
type ShellConfirmer interface {
	ConfirmCommand(command string) (bool, error)
}

// contextKey type for context values
type contextKey string

const shellConfirmerKey contextKey = "shell_confirmer"

// WithShellConfirmer adds a shell confirmer to context
func WithShellConfirmer(ctx context.Context, confirmer ShellConfirmer) context.Context {
	return context.WithValue(ctx, shellConfirmerKey, confirmer)
}

// GetShellConfirmer retrieves shell confirmer from context
func GetShellConfirmer(ctx context.Context) (ShellConfirmer, bool) {
	confirmer, ok := ctx.Value(shellConfirmerKey).(ShellConfirmer)
	return confirmer, ok
}

// Tool to execute a command under the optional given working directory.
func Shell() tool.Invoker {
	return tool.NewInvoker(tool.Info{
		Name: "shell",
		Description: fmt.Sprintf(
			"Execute a shell command in %s under the optional given working directory, current working directory will be used if not provided.",
			pkgos.GetSystemDistro(),
		),
	}, func(ctx context.Context, input *ShellInput) (string, error) {
		// Check if command is completely blocked
		if checkCommandBlocked(input.Command) {
			return "", wrapShellError(errDangerousCommand, shellBlockedTag)
		}
		
		// Check if command needs user confirmation
		if checkCommandNeedsConfirmation(input.Command) {
			// Try to get confirmer from context
			confirmer, ok := GetShellConfirmer(ctx)
			if !ok {
				// No confirmer available, return error requesting confirmation
				return "", wrapShellError(&ConfirmationRequiredError{Command: input.Command}, shellConfirmNeededTag)
			}
			
			confirmed, err := confirmer.ConfirmCommand(input.Command)
			if err != nil {
				return "", wrapShellError(fmt.Errorf("confirmation failed: %w", err), shellBlockedTag)
			}
			if !confirmed {
				return "", wrapShellError(errors.New("command execution denied by user"), shellBlockedTag)
			}
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
