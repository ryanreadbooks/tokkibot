//go:build windows

package guard

import "regexp"

// Commands that require user confirmation before execution
var ConfirmRequiredPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bdel\s+`),   // Any del command
	regexp.MustCompile(`(?i)\brmdir\s+`), // Any rmdir command
}

// Commands that are completely blocked (no confirmation)
var DangerousPatterns = []*regexp.Regexp{
	// destructive del / rmdir
	regexp.MustCompile(`(?i)\bdel\s+/[^\s]*[sfq][^\s]*\s+[A-Za-z]:\\\*`),
	regexp.MustCompile(`(?i)\brmdir\s+/[^\s]*s[^\s]*\s+[A-Za-z]:\\$`),
	// disk operations
	regexp.MustCompile(`(?i)\b(format|diskpart)\b`),
	// system power
	regexp.MustCompile(`(?i)\b(shutdown|restart)\b`),
	// fork bomb variant (cmd)
	regexp.MustCompile(`(?i)%0\s*\|\s*%0`),
	// registry tampering
	regexp.MustCompile(`(?i)\breg\s+(delete|add)\s+HKLM\\`),
	// download & execute (IEX / Invoke-Expression)
	regexp.MustCompile(`(?i)\b(Invoke-Expression|iex)\b`),
	regexp.MustCompile(`(?i)\|\s*(cmd|powershell|pwsh)\b`),
}
