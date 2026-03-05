package gateway

import (
	"context"

	"github.com/ryanreadbooks/tokkibot/channel/adapter/lark"
	"github.com/ryanreadbooks/tokkibot/config"
	gw "github.com/ryanreadbooks/tokkibot/gateway"

	"github.com/spf13/cobra"
)

var GatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Start the tokkibot gateway.",
	Long:  "Start the tokkibot gateway.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return initGateway(cmd.Context())
	},
}

func initGateway(ctx context.Context) error {
	g, err := gw.NewGateway(ctx,
		gw.WithVerbose(true),
		gw.WithRunCronTasks(true),
	)
	if err != nil {
		return err
	}
	lark := lark.NewAdapter(lark.LarkConfig{
		AppId:     config.GetConfig().Adapters.Lark.AppId,
		AppSecret: config.GetConfig().Adapters.Lark.AppSecret,
	})

	g.AddAdapter(lark)

	return g.Run(ctx) // block here
}
