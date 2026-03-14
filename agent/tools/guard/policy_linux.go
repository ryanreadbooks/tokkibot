//go:build linux

package guard

import "regexp"

var ConfirmRequiredPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\brm\s+.*`),
}

var DangerousPatterns = []*regexp.Regexp{
	// destructive rm
	regexp.MustCompile(`\brm\s+-[^\s]*(rf|fr)[^\s]*\s+(/|~|\$HOME|/\*)\b`),
	// disk format / dd
	regexp.MustCompile(`\bmkfs\.`),
	regexp.MustCompile(`\bdd\s+if=/dev/(zero|random|urandom)\b`),
	regexp.MustCompile(`>\s*/dev/(sd|nvme)`),
	// system power
	regexp.MustCompile(`\b(shutdown|reboot|poweroff)\b`),
	// fork bomb variants
	regexp.MustCompile(`:\(\)\s*\{.*\};\s*:`),
	regexp.MustCompile(`\./\$0\s*\|\s*\./\$0`),
	// curl/wget pipe to shell
	regexp.MustCompile(`\|\s*(sh|bash|zsh)\b`),
}
