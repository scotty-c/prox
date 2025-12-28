package cmd

import (
	"fmt"

	"github.com/scotty-c/prox/cmd"
	"github.com/spf13/cobra"
)

// ctCmd represents the container command
var ctCmd = &cobra.Command{
	Use:     "ct",
	GroupID: "management",
	Aliases: []string{"container", "lxc"},
	Short:   "Manage LXC containers on Proxmox VE",
	Long:    `Commands for managing LXC containers including create, start, stop, describe, list, templates, and more.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("unknown ct subcommand: %s", args[0])
		}
		return cmd.Help()
	},
	DisableSuggestions:         false,
	SuggestionsMinimumDistance: 1,
}

func init() {
	cmd.RootCmd.AddCommand(ctCmd)
}
