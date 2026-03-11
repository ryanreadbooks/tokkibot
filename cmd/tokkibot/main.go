package main

import (
	"github.com/ryanreadbooks/tokkibot/cmd/agent"
	"github.com/ryanreadbooks/tokkibot/cmd/cron"
	"github.com/ryanreadbooks/tokkibot/cmd/gateway"
	"github.com/ryanreadbooks/tokkibot/cmd/mcp"
	"github.com/ryanreadbooks/tokkibot/cmd/onboard"
	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/pkg/log"
	"github.com/ryanreadbooks/tokkibot/pkg/process"
	"github.com/spf13/cobra"
)

var skipConfigCmds = map[string]bool{
	"onboard": true,
	"mcp":     true,
	"add":     true,
	"list":    true,
	"remove":  true,
	"rm":      true,
}

var rootCmd = &cobra.Command{
	Use: "tokkibot",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if skipConfigCmds[cmd.Name()] {
			return
		}
		config.MustInit()
	},
}

func init() {
	rootCmd.AddCommand(agent.AgentCmd)
	rootCmd.AddCommand(onboard.OnboardCmd)
	rootCmd.AddCommand(gateway.GatewayCmd)
	rootCmd.AddCommand(cron.CronCmd)
	rootCmd.AddCommand(mcp.McpCmd)
}

func main() {
	ctx, cancel, wait := process.GetRootContext()
	rootCmd.ExecuteContext(ctx)
	cancel()

	wait()

	// Close logger on exit
	log.Close()
}
