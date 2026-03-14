//go:build darwin

package guard

import "regexp"

var ConfirmRequiredPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\brm\s+.*`),
}

var DangerousPatterns = []*regexp.Regexp{
	// destructive rm
	regexp.MustCompile(`\brm\s+-[^\s]*(rf|fr)[^\s]*\s+(/|~|\$HOME|/\*)\b`),
	// macOS disk utility
	regexp.MustCompile(`\bdiskutil\s+(eraseDisk|partitionDisk|secureErase)\b`),
	// dd
	regexp.MustCompile(`\bdd\s+if=/dev/(zero|random|urandom)\b`),
	regexp.MustCompile(`>\s*/dev/disk`),
	// system power
	regexp.MustCompile(`\b(shutdown|reboot)\b`),
	// SIP / firmware tampering
	regexp.MustCompile(`\bcsrutil\s+disable\b`),
	regexp.MustCompile(`\bnvram\b`),
	// fork bomb variants
	regexp.MustCompile(`:\(\)\s*\{.*\};\s*:`),
	regexp.MustCompile(`\./\$0\s*\|\s*\./\$0`),
	// curl/wget pipe to shell
	regexp.MustCompile(`\|\s*(sh|bash|zsh)\b`),
}
