//go:build windows

package tools

import "regexp"

// Commands that require user confirmation before execution
var confirmRequiredPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bdel\s+`),   // Any del command
	regexp.MustCompile(`(?i)\brmdir\s+`), // Any rmdir command
}

// Commands that are completely blocked (no confirmation)
var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(format|diskpart)\b`),  // disk operations
	regexp.MustCompile(`(?i)\b(shutdown|restart)\b`), // system power
}
