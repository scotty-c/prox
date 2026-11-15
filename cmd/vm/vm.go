package cmd

import (
	"github.com/scotty-c/prox/cmd"
	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/spf13/cobra"
)

// vmCmd represents the root vm command

var vmCmd = &cobra.Command{
	Use:     "vm",
	GroupID: "management",
	Short:   "Manage virtual machines on Proxmox VE",
	Long:    `Commands for managing virtual machines including start, stop, clone, delete, and more.`,
}

func init() {
	cmdutil.MakeGroup(vmCmd)
	cmd.RootCmd.AddCommand(vmCmd)
}
