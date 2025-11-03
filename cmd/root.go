package cmd

import (
	"github.com/extremtechniker/godns/logger"
	"github.com/spf13/cobra"
)

var LogLevel string

func RootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "godns",
		Short: "Go DNS server with Postgres and Redis",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			logger.InitLogger(LogLevel)
		},
	}
	root.PersistentFlags().StringVar(&LogLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	return root
}
