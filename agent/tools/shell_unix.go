//go:build !windows

package tools

import "regexp"

var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\brm\s+-[rf]{1,2}\b`),            // rm -r, rm -rf, rm -fr
	regexp.MustCompile(`\b(format|mkfs)\b`),              // disk format
	regexp.MustCompile(`\bdd\s+if=`),                     // dd
	regexp.MustCompile(`>\s*/dev/sd`),                    // write to disk
	regexp.MustCompile(`\b(shutdown|reboot|poweroff)\b`), // system power
	regexp.MustCompile(`:\(\)\s*\{.*\};\s*:`),            // fork bomb
}
