//go:build windows

package tools

import "regexp"

var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bdel\s+/[fq]\b`),        // del /f, del /q
	regexp.MustCompile(`(?i)\brmdir\s+/s\b`),         // rmdir /s
	regexp.MustCompile(`(?i)\b(format|diskpart)\b`),  // disk operations
	regexp.MustCompile(`(?i)\b(shutdown|restart)\b`), // system power
}
