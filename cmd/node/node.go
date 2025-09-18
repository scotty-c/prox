package node

import (
	"github.com/spf13/cobra"
)

// NodeCmd represents the base command for node operations
var NodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Manage Proxmox nodes",
	Long:  `Manage Proxmox nodes in the datacenter.`,
}

func init() {
	// Subcommands will be added in their own files
}
