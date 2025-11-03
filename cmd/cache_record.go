package cmd

import (
	"context"
	"strings"

	"github.com/extremtechniker/godns/cache"
	"github.com/extremtechniker/godns/db"
	"github.com/extremtechniker/godns/logger"
	"github.com/spf13/cobra"
)

func CacheRecordCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache-record <domain> <type>",
		Short: "Pre-warm Redis cache for a specific record (domain + qtype)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := db.InitPostgres(ctx); err != nil {
				return err
			}
			if err := cache.InitRedis(ctx); err != nil {
				return err
			}

			domain := args[0]
			qtype := strings.ToUpper(args[1])
			recs, err := db.FetchRecords(ctx, domain, qtype)
			if err != nil {
				return err
			}

			if err := cache.CacheRecord(ctx, domain, qtype, recs); err != nil {
				return err
			}

			logger.Logger.Infof("Cached record: %s %s", domain, qtype)
			return nil
		},
	}
	return cmd
}
