package cmd

import (
	"context"

	"github.com/extremtechniker/godns/api"
	"github.com/extremtechniker/godns/cache"
	"github.com/extremtechniker/godns/db"
	"github.com/extremtechniker/godns/dns"
	"github.com/extremtechniker/godns/logger"
	"github.com/extremtechniker/godns/util"
	"github.com/spf13/cobra"
)

func DaemonCommand() *cobra.Command {
	var httpAPI bool

	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run DNS server daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Initialize dependencies
			if err := db.InitPostgres(ctx); err != nil {
				return err
			}
			if err := cache.InitRedis(ctx); err != nil {
				return err
			}

			// Optional HTTP API
			if httpAPI {
				go func() {
					err := api.StartServer(ctx)
					if err != nil {
						logger.Logger.Fatal(err)
						return
					}
				}()
			}

			listen := util.MustGetenv("DNS_LISTEN", ":1053")
			return dns.RunDaemon(ctx, listen)
		},
	}

	cmd.Flags().BoolVar(&httpAPI, "http-api", false, "Start HTTP API alongside DNS daemon")
	return cmd
}
