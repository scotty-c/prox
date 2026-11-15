package cmd

import (
	"github.com/scotty-c/prox/cmd"
	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/spf13/cobra"
)

// ctCmd represents the container command
var ctCmd = &cobra.Command{
	Use:                        "ct",
	GroupID:                    "management",
	Aliases:                    []string{"container", "lxc"},
	Short:                      "Manage LXC containers on Proxmox VE",
	Long:                       `Commands for managing LXC containers including create, start, stop, describe, list, templates, and more.`,
	SuggestionsMinimumDistance: 1,
}

func init() {
	cmdutil.MakeGroup(ctCmd)
	cmd.RootCmd.AddCommand(ctCmd)
}
