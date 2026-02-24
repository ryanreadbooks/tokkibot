//go:build !windows

package tools

import "regexp"

// Commands that require user confirmation before execution
var confirmRequiredPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\brm\s+.*`), // Any rm command
}

// Commands that are completely blocked (no confirmation)
var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\b(format|mkfs)\b`),              // disk format
	regexp.MustCompile(`\bdd\s+if=`),                     // dd
	regexp.MustCompile(`>\s*/dev/sd`),                    // write to disk
	regexp.MustCompile(`\b(shutdown|reboot|poweroff)\b`), // system power
	regexp.MustCompile(`:\(\)\s*\{.*\};\s*:`),            // fork bomb
}
