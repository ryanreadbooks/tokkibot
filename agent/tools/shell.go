package tools

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ryanreadbooks/tokkibot/component/tool"
	"github.com/ryanreadbooks/tokkibot/pkg/os"
)

const (
	maxAllowedShellOutputLen = 15000
)

type shellResultTag string

const (
	shellBlockedTag shellResultTag = "<shell_blocked>"
	shellRunErrTag  shellResultTag = "<shell_run_error>"
	shellStdoutTag  shellResultTag = "<shell_stdout>"
	shellStderrTag  shellResultTag = "<shell_stderr>"
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

// parseCommand parses a command string into command name and arguments.
// It handles quoted arguments (single and double quotes) and escaped characters.
func parseCommand(input string) (name string, args []string) {
	var tokens []string
	var current strings.Builder
	var inSingleQuote, inDoubleQuote, escaped bool

	for _, r := range input {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}

		switch r {
		case '\\':
			if inSingleQuote {
				current.WriteRune(r)
			} else {
				escaped = true
			}
		case '\'':
			if inDoubleQuote {
				current.WriteRune(r)
			} else {
				inSingleQuote = !inSingleQuote
			}
		case '"':
			if inSingleQuote {
				current.WriteRune(r)
			} else {
				inDoubleQuote = !inDoubleQuote
			}
		case ' ', '\t':
			if inSingleQuote || inDoubleQuote {
				current.WriteRune(r)
			} else if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	if len(tokens) == 0 {
		return "", nil
	}
	
	return tokens[0], tokens[1:]
}

// Tool to execute a command under the optional given working directory.
func Shell() tool.Invoker {
	return tool.NewInvoker(tool.Info{
		Name: "shell",
		Description: fmt.Sprintf(
			"Execute a shell command in %s under the optional given working directory, current working directory will be used if not provided",
			os.GetSystemDistro(),
		),
	}, func(ctx context.Context, input *ShellInput) (string, error) {
		err := safeCheckShellCommand(input.Command)
		if err != nil {
			return "", wrapShellError(err, shellBlockedTag)
		}

		name, args := parseCommand(input.Command)
		if name == "" {
			return "", wrapShellError(errors.New("empty command"), shellBlockedTag)
		}
		cmd := exec.CommandContext(ctx, name, args...)
		if input.WorkingDir != "" {
			cleanWd, _ := resolvePath(input.WorkingDir, "")
			cmd.Dir = cleanWd
		}

		// redirect stdout and stderr
		var redirectStdout, redirectStderr strings.Builder
		cmd.Stdout = &redirectStdout
		cmd.Stderr = &redirectStderr

		err = cmd.Run()
		if err != nil {
			return "", wrapShellError(err, shellRunErrTag)
		}

		var combinedBuilder strings.Builder
		stdout := redirectStdout.String()
		stderr := redirectStderr.String()
		combinedBuilder.Grow(len(stdout) + len(stderr))
		if len(stdout) > 0 {
			combinedBuilder.WriteString(string(shellStdoutTag))
			combinedBuilder.WriteString(stdout)
			combinedBuilder.WriteString(string(shellStdoutTag))
			combinedBuilder.WriteString("\n")
		}
		if len(stderr) > 0 {
			combinedBuilder.WriteString(string(shellStderrTag))
			combinedBuilder.WriteString(stderr)
			combinedBuilder.WriteString(string(shellStderrTag))
			combinedBuilder.WriteString("\n")
		}

		combinedOutput := combinedBuilder.String()

		// we may need to truncated the final output
		if lenCombinedOutput := len(combinedOutput); lenCombinedOutput > maxAllowedShellOutputLen {
			more := lenCombinedOutput - maxAllowedShellOutputLen
			combinedOutput = combinedOutput[:maxAllowedShellOutputLen] + fmt.Sprintf("\n... (truncated, %d more chars)", more)
		}

		return combinedOutput, nil
	})
}
