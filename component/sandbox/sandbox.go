package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

type option struct {
	// read only
	readOnlyPaths []string

	// read and write
	readWritePaths []string

	// initial working directory inside sandbox
	workingDir string
}

type Option func(*option)

func WithReadOnlyPaths(paths ...string) Option {
	return func(o *option) {
		o.readOnlyPaths = paths
	}
}

func WithReadWritePaths(paths ...string) Option {
	return func(o *option) {
		o.readWritePaths = paths
	}
}

func WithWorkingDir(dir string) Option {
	return func(o *option) {
		o.workingDir = dir
	}
}

// SandboxError indicates the sandbox itself failed (permission, mount, config issues).
// The command never ran. Retrying the same command won't help.
type SandboxError struct {
	Reason string
	Err    error
}

func (e *SandboxError) Error() string { return fmt.Sprintf("sandbox error: %s: %v", e.Reason, e.Err) }
func (e *SandboxError) Unwrap() error { return e.Err }

// CommandError indicates the command ran inside the sandbox but exited with non-zero code.
// The output may contain useful diagnostic information.
type CommandError struct {
	ExitCode int
	Output   string
	Err      error
}

func (e *CommandError) Error() string {
	return fmt.Sprintf("command exited with code %d: %s", e.ExitCode, e.Output)
}
func (e *CommandError) Unwrap() error { return e.Err }

func IsSandboxError(err error) bool {
	var se *SandboxError
	return errors.As(err, &se)
}

func IsCommandError(err error) bool {
	var ce *CommandError
	return errors.As(err, &ce)
}

type Sandbox interface {
	Execute(ctx context.Context, command string) (string, error)
}

// PassthroughSandbox executes commands directly on the host without isolation.
type PassthroughSandbox struct {
	workingDir string
}

func NewPassthroughSandbox(workingDir string) Sandbox {
	return &PassthroughSandbox{workingDir: workingDir}
}

func (p *PassthroughSandbox) Execute(ctx context.Context, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	if p.workingDir != "" {
		cmd.Dir = p.workingDir
	}

	output, err := cmd.CombinedOutput()
	if err == nil {
		return string(output), nil
	}

	exitCode := -1
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}

	return "", &CommandError{
		ExitCode: exitCode,
		Output:   string(output),
		Err:      err,
	}
}
