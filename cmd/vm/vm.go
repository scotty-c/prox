package cmd

import (
	"github.com/scotty-c/prox/cmd"
	"github.com/spf13/cobra"
)

// vmCmd represents the root vm command

var vmCmd = &cobra.Command{
	Use:   "vm",
	Short: "Manage virtual machines on Proxmox VE",
	Long:  `Commands for managing virtual machines including start, stop, clone, delete, and more.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	cmd.RootCmd.AddCommand(vmCmd)
}
