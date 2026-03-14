package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"runtime"
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

// getSystemShell returns the shell executable and arguments for the current OS.
// Returns (shell, flag) where the command should be executed as: shell flag "command"
func getSystemShell() (shell string, flag string) {
	switch runtime.GOOS {
	case "windows":
		return "cmd", "/C"
	default:
		// Linux, macOS, and other Unix-like systems
		return "sh", "-c"
	}
}

// isCurlCommand checks if the command starts with curl
func isCurlCommand(command string) bool {
	cmd, _ := bash.ParseCommand(command)
	return strings.HasPrefix(cmd, "curl ") || cmd == "curl"
}

// isHTMLContent checks if the content appears to be HTML
func isHTMLContent(content string) bool {
	trimmed := strings.TrimSpace(content)
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(lower, "<!doctype html") ||
		strings.HasPrefix(lower, "<html") ||
		strings.Contains(lower, "<head>") ||
		strings.Contains(lower, "<body")
}

var (
	// Match <style>...</style> tags (including attributes)
	styleTagRe = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	// Match <link rel="stylesheet" ...> tags
	linkStylesheetRe = regexp.MustCompile(`(?i)<link[^>]*rel\s*=\s*["']?stylesheet["']?[^>]*>`)
	// Match inline style attributes: style="..." or style='...'
	inlineStyleRe = regexp.MustCompile(`(?i)\s+style\s*=\s*["'][^"']*["']`)
	// Match <script>...</script> tags
	scriptTagRe = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	// Match HTML comments
	htmlCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)
	// Match multiple consecutive newlines/whitespace
	multiNewlineRe = regexp.MustCompile(`\n\s*\n\s*\n+`)
)

// filterHTMLContent removes CSS and unnecessary content from HTML to reduce token usage
func filterHTMLContent(content string) string {
	result := content

	// Remove <style> tags
	result = styleTagRe.ReplaceAllString(result, "")
	// Remove <link rel="stylesheet"> tags
	result = linkStylesheetRe.ReplaceAllString(result, "")
	// Remove inline style attributes
	result = inlineStyleRe.ReplaceAllString(result, "")
	// Remove <script> tags
	result = scriptTagRe.ReplaceAllString(result, "")
	// Remove HTML comments
	result = htmlCommentRe.ReplaceAllString(result, "")
	// Collapse multiple newlines into two
	result = multiNewlineRe.ReplaceAllString(result, "\n\n")

	return strings.TrimSpace(result)
}

func doShellInvoke(ctx context.Context, meta tool.InvokeMeta, input *ShellInput) (string, error) {
	if strings.TrimSpace(input.Command) == "" {
		return "", wrapShellError(errors.New("empty command"), shellRunErrTag)
	}

	if tmpCmd, _ := bash.ParseCommand(input.Command); tmpCmd == "" {
		return "", wrapShellError(errors.New("invalid command"), shellRunErrTag)
	}

	shell, flag := getSystemShell()
	ctx, cancel := context.WithTimeout(ctx, shellExecTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, shell, flag, input.Command)
	if input.WorkingDir != "" {
		cleanWd, _ := guard.ResolvePath(input.WorkingDir, []string{})
		cmd.Dir = cleanWd
	}

	startTime := time.Now()
	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		slog.WarnContext(ctx, "[tool/shell] command failed",
			slog.String("command", input.Command),
			slog.Int64("duration_ms", duration),
			slog.Any("error", err))
		return "", wrapShellError(fmt.Errorf("%w: %s", err, string(output)), shellRunErrTag)
	}

	outputStr := string(output)

	// Filter HTML content if this is a curl command returning HTML the content will be extremely large
	if isCurlCommand(input.Command) && isHTMLContent(outputStr) {
		outputStr = filterHTMLContent(outputStr)
	}

	// truncate output
	if len(outputStr) > maxAllowedShellOutputLen {
		more := len(outputStr) - maxAllowedShellOutputLen
		outputStr = outputStr[:maxAllowedShellOutputLen] + fmt.Sprintf("\n... (truncated, %d more chars)", more)
	}

	return outputStr, nil
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
