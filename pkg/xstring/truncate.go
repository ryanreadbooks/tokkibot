package xstring

import "unicode/utf8"

func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}

	count := 0
	index := 0
	for index < len(s) {
		if count >= maxLen {
			break
		}

		_, size := utf8.DecodeRuneInString(s[index:])
		index += size
		count++
	}

	return s[:index]
}
