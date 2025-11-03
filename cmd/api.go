package cmd

import (
	"context"

	"github.com/extremtechniker/godns/api"
	"github.com/extremtechniker/godns/db"
	"github.com/spf13/cobra"
)

func ApiCommand() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "api",
		Short: "Start HTTP API for managing records",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if err := db.InitPostgres(ctx); err != nil {
				return err
			}

			return api.StartServer(ctx)
		},
	}

	return cmd
}
