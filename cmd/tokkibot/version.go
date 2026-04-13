package main

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	version string = "dev"
	commit  string = "unknown"
)

var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of tokkibot",
	Run: func(cmd *cobra.Command, args []string) {
		platform := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
		cmd.Println(fmt.Sprintf("tokkibot %s (commit: %s, platform: %s)", version, commit, platform))
	},
}
