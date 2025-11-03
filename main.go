package main

import (
	"github.com/extremtechniker/godns/cmd"
)

func main() {
	root := cmd.RootCommand()
	root.AddCommand(cmd.DaemonCommand())
	root.AddCommand(cmd.AddRecordCommand())
	root.AddCommand(cmd.CacheRecordCommand())
	root.AddCommand(cmd.TokenCommand())
	root.AddCommand(cmd.ApiCommand())

	if err := root.Execute(); err != nil {
		panic(err)
	}
}
