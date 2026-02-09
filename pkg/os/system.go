package os

import (
	"os"
	"runtime"
	"strings"
)

func GetSystemDistro() string {
	switch runtime.GOOS {
	case "linux":
		data, err := os.ReadFile("/etc/os-release")
		if err != nil {
			return "linux"
		}
		for _, line := range strings.Split(string(data), "\n") {
			if after, ok := strings.CutPrefix(line, "ID="); ok {
				return strings.Trim(after, `"`)
			}
		}
		return "linux"
	case "darwin":
		return "macos"
	case "windows":
		return "windows"
	default:
		return runtime.GOOS
	}
}
