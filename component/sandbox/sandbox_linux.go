//go:build linux

package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ryanreadbooks/tokkibot/pkg/bash"
)

const bwrapErrorPrefix = "bwrap:"

// Lite-weight sandbox for linux based on bubblewrap
//
// usage: bwrap [OPTIONS...] [--] COMMAND [ARGS...]
//
//     --help                       Print this help
//     --version                    Print version
//     --args FD                    Parse NUL-separated args from FD
//     --unshare-all                Unshare every namespace we support by default
//     --share-net                  Retain the network namespace (can only combine with --unshare-all)
//     --unshare-user               Create new user namespace (may be automatically implied if not setuid)
//     --unshare-user-try           Create new user namespace if possible else continue by skipping it
//     --unshare-ipc                Create new ipc namespace
//     --unshare-pid                Create new pid namespace
//     --unshare-net                Create new network namespace
//     --unshare-uts                Create new uts namespace
//     --unshare-cgroup             Create new cgroup namespace
//     --unshare-cgroup-try         Create new cgroup namespace if possible else continue by skipping it
//     --userns FD                  Use this user namespace (cannot combine with --unshare-user)
//     --userns2 FD                 After setup switch to this user namspace, only useful with --userns
//     --pidns FD                   Use this user namespace (as parent namespace if using --unshare-pid)
//     --uid UID                    Custom uid in the sandbox (requires --unshare-user or --userns)
//     --gid GID                    Custom gid in the sandbox (requires --unshare-user or --userns)
//     --hostname NAME              Custom hostname in the sandbox (requires --unshare-uts)
//     --chdir DIR                  Change directory to DIR
//     --setenv VAR VALUE           Set an environment variable
//     --unsetenv VAR               Unset an environment variable
//     --lock-file DEST             Take a lock on DEST while sandbox is running
//     --sync-fd FD                 Keep this fd open while sandbox is running
//     --bind SRC DEST              Bind mount the host path SRC on DEST
//     --bind-try SRC DEST          Equal to --bind but ignores non-existent SRC
//     --dev-bind SRC DEST          Bind mount the host path SRC on DEST, allowing device access
//     --dev-bind-try SRC DEST      Equal to --dev-bind but ignores non-existent SRC
//     --ro-bind SRC DEST           Bind mount the host path SRC readonly on DEST
//     --ro-bind-try SRC DEST       Equal to --ro-bind but ignores non-existent SRC
//     --bind-fd FD DEST            Bind open directory or path fd on DEST
//     --ro-bind-fd FD DEST         Bind open directory or path fd read-only on DEST
//     --remount-ro DEST            Remount DEST as readonly; does not recursively remount
//     --exec-label LABEL           Exec label for the sandbox
//     --file-label LABEL           File label for temporary sandbox content
//     --proc DEST                  Mount new procfs on DEST
//     --dev DEST                   Mount new dev on DEST
//     --tmpfs DEST                 Mount new tmpfs on DEST
//     --mqueue DEST                Mount new mqueue on DEST
//     --dir DEST                   Create dir at DEST
//     --file FD DEST               Copy from FD to destination DEST
//     --bind-data FD DEST          Copy from FD to file which is bind-mounted on DEST
//     --ro-bind-data FD DEST       Copy from FD to file which is readonly bind-mounted on DEST
//     --symlink SRC DEST           Create symlink at DEST with target SRC
//     --seccomp FD                 Load and use seccomp rules from FD
//     --block-fd FD                Block on FD until some data to read is available
//     --userns-block-fd FD         Block on FD until the user namespace is ready
//     --info-fd FD                 Write information about the running container to FD
//     --json-status-fd FD          Write container status to FD as multiple JSON documents
//     --new-session                Create a new terminal session
//     --die-with-parent            Kills with SIGKILL child process (COMMAND) when bwrap or bwrap's parent dies.
//     --as-pid-1                   Do not install a reaper process with PID=1
//     --cap-add CAP                Add cap CAP when running as privileged user
//     --cap-drop CAP               Drop cap CAP when running as privileged user

type SandboxImpl struct {
	opt option
}

func NewSandbox(opts ...Option) Sandbox {
	opt := option{}
	for _, o := range opts {
		o(&opt)
	}

	return &SandboxImpl{opt: opt}
}

var _ Sandbox = (*SandboxImpl)(nil)

func (s *SandboxImpl) beforeExecute(_ context.Context) (string, error) {
	bwrapPath, err := exec.LookPath("bwrap") // bubblewrap
	if err != nil {
		return "", fmt.Errorf("bwrap not found: %w", err)
	}

	return bwrapPath, nil
}

func (s *SandboxImpl) Execute(ctx context.Context, command string) (string, error) {
	bwrapPath, err := s.beforeExecute(ctx)
	if err != nil {
		return "", &SandboxError{Reason: "bwrap not available", Err: err}
	}

	executableToken, _ := bash.ParseCommand(command)
	if executableToken == "" {
		return "", &SandboxError{Reason: "empty command", Err: fmt.Errorf("no executable parsed from command")}
	}

	executablePath, err := resolveExecutablePath(executableToken)
	if err != nil {
		return "", &SandboxError{Reason: fmt.Sprintf("executable %q not found on host", executableToken), Err: err}
	}

	bwrapArgs := s.buildBwrapArgs(executablePath)
	bwrapArgs = append(bwrapArgs, "sh", "-c", command)

	cmd := exec.CommandContext(ctx, bwrapPath, bwrapArgs...)
	// Explicitly pass host environment variables into bwrap process.
	// bwrap --setenv flags in buildBwrapArgs can still override selected keys.
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err == nil {
		return string(output), nil
	}

	outputStr := string(output)
	exitCode := -1
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}

	if isBwrapError(outputStr) {
		return "", &SandboxError{
			Reason: fmt.Sprintf("sandbox restriction (exit %d): %s", exitCode, summarizeOutput(outputStr)),
			Err:    err,
		}
	}

	return "", &CommandError{
		ExitCode: exitCode,
		Output:   outputStr,
		Err:      err,
	}
}

func isBwrapError(output string) bool {
	return strings.Contains(output, bwrapErrorPrefix)
}

func summarizeOutput(output string) string {
	const maxLen = 200
	output = strings.TrimSpace(output)
	if len(output) > maxLen {
		return output[:maxLen] + "..."
	}
	return output
}

func resolveExecutablePath(token string) (string, error) {
	executable := normalizeExecutableToken(token)
	if executable == "" {
		return "", fmt.Errorf("empty executable token")
	}

	path, err := exec.LookPath(executable)
	if err == nil {
		return path, nil
	}

	// Shell builtins (e.g. cd/export/source) are resolved by sh at runtime.
	// For these commands, bind shell itself instead of failing early.
	if isShellBuiltin(executable) {
		shPath, shErr := exec.LookPath("sh")
		if shErr != nil {
			return "", fmt.Errorf("shell not found: %w", shErr)
		}
		return shPath, nil
	}

	return "", err
}

func normalizeExecutableToken(token string) string {
	s := strings.TrimSpace(token)
	for strings.HasPrefix(s, "(") {
		s = strings.TrimPrefix(s, "(")
	}
	return strings.TrimSpace(s)
}

func isShellBuiltin(name string) bool {
	switch name {
	case "cd", ".", "source", "export", "unset", "set", "alias", "unalias",
		"readonly", "local", "shift", "eval", "exec", "test", "[", "umask",
		"ulimit", "wait", "trap", "history", "jobs", "fg", "bg":
		return true
	default:
		return false
	}
}

const sandboxHome = "/home/sandbox"

func (s *SandboxImpl) buildBwrapArgs(executablePath string) []string {
	hostHome, _ := os.UserHomeDir()

	args := []string{
		"--unshare-all",
		"--share-net",
		"--die-with-parent",
		"--new-session",
	}

	// ── 1. System core directories ── read only
	for _, path := range []string{"/usr", "/lib", "/lib64", "/bin", "/sbin"} {
		if pathExists(path) {
			args = append(args, "--ro-bind", path, path)
		}
	}

	// ── 2. Minimal /etc entries ── read only
	for _, path := range []string{
		"/etc/ld.so.cache",
		"/etc/ld.so.conf",
		"/etc/ld.so.conf.d",
		"/etc/alternatives",
		"/etc/ssl/certs",
		"/etc/ca-certificates",
		"/etc/resolv.conf",
		"/etc/hosts",
		"/etc/nsswitch.conf",
		"/etc/passwd",
		"/etc/group",
	} {
		if pathExists(path) {
			args = append(args, "--ro-bind", path, path)
		}
	}

	// ── 3. Virtual filesystems
	args = append(args,
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--tmpfs", "/run",
		"--tmpfs", "/var/tmp",
	)

	// ── 4. Writable sandbox HOME (tmpfs)
	// Must come BEFORE tool chain ro-binds so they can overlay on top
	args = append(args, "--tmpfs", sandboxHome)

	// ── 5. Host script tool chains ── read only (auto-detected, overlays tmpfs HOME)
	for _, dir := range resolveToolChainDirs() {
		args = append(args, "--ro-bind", dir, dir)
	}

	// ── 6. Host package caches ── read only (accelerate downloads)
	type cacheMount struct {
		hostPath, sandboxPath, envVar string
	}
	for _, cm := range []cacheMount{
		{hostHome + "/.cache/pip", "/var/cache/pip", "PIP_CACHE_DIR"},
		{hostHome + "/.cache/uv", "/var/cache/uv", "UV_CACHE_DIR"},
		{hostHome + "/.npm/_cacache", "/var/cache/npm", "npm_config_cache"},
	} {
		if pathExists(cm.hostPath) {
			args = append(args, "--ro-bind", cm.hostPath, cm.sandboxPath)
			args = append(args, "--setenv", cm.envVar, cm.sandboxPath)
		}
	}

	// ── 7. Environment variables
	sandboxPaths := strings.Join([]string{
		sandboxHome + "/.local/bin",
		sandboxHome + "/.npm-global/bin",
	}, ":")
	args = append(args,
		"--setenv", "HOME", sandboxHome,
		"--setenv", "PATH", sandboxPaths+":"+os.Getenv("PATH"),
		"--setenv", "PIP_USER", "1",
		"--setenv", "PYTHONUSERBASE", sandboxHome+"/.local",
		"--setenv", "npm_config_prefix", sandboxHome+"/.npm-global",
	)

	// ── 8. User-specified mounts
	for _, path := range s.opt.readOnlyPaths {
		if pathExists(path) {
			args = append(args, "--ro-bind", path, path)
		}
	}
	for _, path := range s.opt.readWritePaths {
		if pathExists(path) {
			args = append(args, "--bind", path, path)
		}
	}

	args = append(args, "--ro-bind", executablePath, executablePath)

	// ── 9. Working directory
	if s.opt.workingDir != "" {
		args = append(args, "--chdir", s.opt.workingDir)
	}

	return args
}

var defaultScriptTools = []string{
	"python3", "python", "pip3", "pip",
	"node", "npm", "npx",
	"uv", "uvx",
}

// resolveToolChainDirs discovers installation prefixes for common script tools.
//
// For each tool, it resolves symlinks to find the real binary location,
// then walks up from <prefix>/bin/<tool> to mount <prefix>/ (which typically
// contains bin/, lib/, include/, etc.).
//
// Paths already covered by system read-only mounts (/usr, /bin, ...) are skipped.
func resolveToolChainDirs() []string {
	systemPrefixes := []string{"/usr", "/lib", "/lib64", "/bin", "/sbin"}

	seen := make(map[string]bool)
	var dirs []string

	for _, tool := range defaultScriptTools {
		binPath, err := exec.LookPath(tool)
		if err != nil {
			continue
		}

		realPath, err := filepath.EvalSymlinks(binPath)
		if err != nil {
			realPath = binPath
		}

		// <prefix>/bin/<tool> → <prefix>
		prefix := filepath.Dir(filepath.Dir(realPath))

		covered := false
		for _, sys := range systemPrefixes {
			if prefix == sys || strings.HasPrefix(prefix, sys+"/") {
				covered = true
				break
			}
		}
		if covered || seen[prefix] {
			continue
		}

		seen[prefix] = true
		dirs = append(dirs, prefix)
	}

	return dirs
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
