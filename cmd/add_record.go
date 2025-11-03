package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/extremtechniker/godns/db"
	"github.com/extremtechniker/godns/logger"
	"github.com/extremtechniker/godns/model"
	"github.com/spf13/cobra"
)

func AddRecordCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-record <domain> <type> <value> [ttl]",
		Short: "Add a DNS record to Postgres",
		Args:  cobra.RangeArgs(3, 4),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := db.InitPostgres(ctx); err != nil {
				return err
			}

			domain := args[0]
			qtype := strings.ToUpper(args[1])
			value := args[2]
			ttl := 300
			if len(args) == 4 {
				fmt.Sscanf(args[3], "%d", &ttl)
			}

			rec := model.Record{Domain: domain, QType: qtype, TTL: ttl, Value: value}
			if err := db.AddRecord(ctx, rec); err != nil {
				return err
			}

			logger.Logger.Infof("Record added: %s %s %s", domain, qtype, value)
			return nil
		},
	}
	return cmd
}
