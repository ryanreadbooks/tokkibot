package main

import (
	"context"

	"github.com/ryanreadbooks/tokkibot/cmd/agent"
	"github.com/ryanreadbooks/tokkibot/cmd/onboard"
	"github.com/ryanreadbooks/tokkibot/config"
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rootCmd.ExecuteContext(ctx)
}
