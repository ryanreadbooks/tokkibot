package main

import (
	"github.com/ryanreadbooks/tokkibot/cmd/agent"
	"github.com/ryanreadbooks/tokkibot/cmd/onboard"
	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/pkg/process"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use: "tokkibot",
}

func init() {
	config.Init()

	rootCmd.AddCommand(agent.AgentCmd)
	rootCmd.AddCommand(onboard.OnboardCmd)
}

func main() {
	ctx, cancel, wait := process.GetRootContext()
	rootCmd.ExecuteContext(ctx)
	cancel()

	wait()
}
