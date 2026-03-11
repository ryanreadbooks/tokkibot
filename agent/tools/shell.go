package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/ryanreadbooks/tokkibot/agent/tools/description"
	"github.com/ryanreadbooks/tokkibot/agent/tools/guard"
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
	for _, p := range guard.ConfirmRequiredPatterns {
		if p.MatchString(command) {
			return true
		}
	}
	return false
}

// checkCommandBlocked checks if command is completely blocked
func checkCommandBlocked(command string) bool {
	for _, p := range guard.DangerousPatterns {
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
		slog.WarnContext(ctx, "[tool/shell] empty command blocked")
		return "", wrapShellError(errors.New("empty command"), shellBlockedTag)
	}

	slog.InfoContext(ctx, "[tool/shell] executing command", slog.String("command", name), slog.String("working_dir", input.WorkingDir))

	ctx, cancel := context.WithTimeout(ctx, shellExecTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	if input.WorkingDir != "" {
		cleanWd, _ := guard.ResolvePath(input.WorkingDir, []string{})
		cmd.Dir = cleanWd
	}

	startTime := time.Now()
	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		slog.WarnContext(ctx, "[tool/shell] command failed", slog.String("command", name), slog.Int64("duration_ms", duration), slog.Any("error", err))
		return "", wrapShellError(fmt.Errorf("%w: %s", err, string(output)), shellRunErrTag)
	}

	outputStr := string(output)
	slog.InfoContext(ctx, "[tool/shell] command completed", slog.String("command", name), slog.Int64("duration_ms", duration), slog.Int("output_len", len(outputStr)))

	// truncate output
	if len(outputStr) > maxAllowedShellOutputLen {
		more := len(output) - maxAllowedShellOutputLen
		outputStr = outputStr[:maxAllowedShellOutputLen] + fmt.Sprintf("\n... (truncated, %d more chars)", more)
		slog.DebugContext(ctx, "[tool/shell] output truncated", slog.Int("truncated_chars", more))
	}

	return string(output), nil
}

func beforeDoShellInvoke(ctx context.Context, meta tool.InvokeMeta, input *ShellInput) error {
	input.Command = strings.TrimSpace(input.Command)
	// Check if command is completely blocked
	if checkCommandBlocked(input.Command) {
		slog.WarnContext(ctx, "[tool/shell] dangerous command blocked", slog.String("command", input.Command))
		return wrapShellError(errDangerousCommand, shellBlockedTag)
	}

	// Check if command needs user confirmation
	if checkCommandNeedsConfirmation(input.Command) {
		slog.InfoContext(ctx, "[tool/shell] command requires confirmation", slog.String("command", input.Command))
		confirmer, ok := tool.GetConfirmer(ctx)
		if !ok {
			slog.WarnContext(ctx, "[tool/shell] no confirmer available, rejecting command")
			return wrapShellError(errConfirmNeeded, shellConfirmNeededTag)
		}

		resp, err := confirmer.RequestConfirm(ctx, &tool.ConfirmRequest{
			Channel:     meta.Channel,
			ChatId:      meta.ChatId,
			ToolName:    ToolNameShell,
			Level:       tool.ConfirmNormal,
			Title:       "Confirm Command Execution",
			Description: "This command may modify system state. Please confirm to proceed.",
			Command:     input.Command,
		})
		if err != nil {
			slog.ErrorContext(ctx, "[tool/shell] confirmation request failed", slog.Any("error", err))
			return wrapShellError(fmt.Errorf("confirmation failed: %w", err), shellConfirmNeededTag)
		}

		if !resp.Confirmed {
			reason := "user rejected"
			if resp.Reason != "" {
				reason = resp.Reason
			}
			slog.InfoContext(ctx, "[tool/shell] command rejected by user", slog.String("reason", reason))
			return wrapShellError(fmt.Errorf("command rejected: %s", reason), shellConfirmNeededTag)
		}
		slog.InfoContext(ctx, "[tool/shell] command confirmed by user")
	}

	return nil
}

// Tool to execute a command under the optional given working directory.
func Shell() tool.Invoker {
	info := tool.Info{
		Name:        ToolNameShell,
		Description: fmt.Sprintf(description.ShellDescription, pkgos.GetSystemDistro()),
	}

	return tool.NewInvoker(info, doShellInvoke, tool.WithBeforeInvoke(beforeDoShellInvoke))
}
