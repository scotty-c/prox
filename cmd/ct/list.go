package cmd

import (
	"github.com/scotty-c/prox/pkg/container"
	"github.com/spf13/cobra"
)

// listCmd represents the list command for containers
var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List LXC containers",
	Long:    `List all LXC containers on Proxmox VE with their status, resource usage, and node information.`,
	Run: func(cmd *cobra.Command, args []string) {
		node, _ := cmd.Flags().GetString("node")
		running, _ := cmd.Flags().GetBool("running")
		container.ListContainers(node, running)
	},
}

func init() {
	listCmd.Flags().StringP("node", "n", "", "Proxmox node name (optional - will show containers from all nodes if not specified)")
	listCmd.Flags().BoolP("running", "r", false, "Show only running containers")
	ctCmd.AddCommand(listCmd)
}
