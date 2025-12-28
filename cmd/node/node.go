// Package node implements CLI commands for managing Proxmox VE nodes.
// It provides commands for listing nodes, describing node details, and performing
// node-level operations in the Proxmox datacenter.
package node

import (
	"github.com/spf13/cobra"
)

// NodeCmd represents the base command for node operations
var NodeCmd = &cobra.Command{
	Use:     "node",
	GroupID: "management",
	Short:   "Manage Proxmox nodes",
	Long:    `Manage Proxmox nodes in the datacenter.`,
}

func init() {
	// Subcommands will be added in their own files
}
