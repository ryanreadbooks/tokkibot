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
	shellBlockedTag       shellResultTag = "<shell_blocked>"
	shellRunErrTag        shellResultTag = "<shell_run_error>"
	shellConfirmNeededTag shellResultTag = "<shell_confirm_needed>"
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

func doShellInvoke(ctx context.Context, meta tool.InvokeMeta, input *ShellInput) (string, error) {
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
}

func beforeDoShellInvoke(ctx context.Context, meta tool.InvokeMeta, input *ShellInput) error {
	// Check if command is completely blocked
	if checkCommandBlocked(input.Command) {
		return wrapShellError(errDangerousCommand, shellBlockedTag)
	}

	// Check if command needs user confirmation
	if checkCommandNeedsConfirmation(input.Command) {
		confirmer, ok := tool.GetConfirmer(ctx)
		if !ok {
			// No confirmer available, reject by default
			return wrapShellError(errConfirmNeeded, shellConfirmNeededTag)
		}

		resp, err := confirmer.RequestConfirm(ctx, &tool.ConfirmRequest{
			Channel:     meta.Channel,
			ChatId:      meta.ChatId,
			ToolName:    "shell",
			Level:       tool.ConfirmNormal,
			Title:       "Confirm Command Execution",
			Description: "This command may modify system state. Please confirm to proceed.",
			Command:     input.Command,
		})
		if err != nil {
			return wrapShellError(fmt.Errorf("confirmation failed: %w", err), shellConfirmNeededTag)
		}

		if !resp.Confirmed {
			reason := "user rejected"
			if resp.Reason != "" {
				reason = resp.Reason
			}
			return wrapShellError(fmt.Errorf("command rejected: %s", reason), shellConfirmNeededTag)
		}
	}

	return nil
}

// Tool to execute a command under the optional given working directory.
func Shell() tool.Invoker {
	info := tool.Info{
		Name: "shell",
		Description: fmt.Sprintf(
			"Execute a shell command in %s under the optional given working directory, current working directory will be used if not provided.",
			pkgos.GetSystemDistro(),
		),
	}

	return tool.NewInvoker(info, doShellInvoke, tool.WithBeforeInvoke(beforeDoShellInvoke))
}
