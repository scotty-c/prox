package cmd

import (
	v "github.com/scotty-c/prox/pkg/vm"
	"github.com/spf13/cobra"
)

// listCmd list vm's on the proxmox server

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List virtual machines on Proxmox VE",
	Long: `List all virtual machines on Proxmox VE with their status, resource usage, and node information.

By default, this command shows basic information quickly. Use --ip to show IP addresses 
and --detailed to get accurate disk usage information (both options are slower).`,
	Run: func(cmd *cobra.Command, args []string) {
		node, _ := cmd.Flags().GetString("node")
		running, _ := cmd.Flags().GetBool("running")
		showIPs, _ := cmd.Flags().GetBool("ip")
		detailed, _ := cmd.Flags().GetBool("detailed")
		jsonOutput, _ := cmd.Flags().GetBool("json")
		v.ListVMs(node, running, showIPs, detailed, jsonOutput)
	},
}

func init() {
	listCmd.Flags().StringP("node", "n", "", "Proxmox node name (optional - will show VMs from all nodes if not specified)")
	listCmd.Flags().BoolP("running", "r", false, "Show only running VMs")
	listCmd.Flags().BoolP("ip", "i", false, "Show IP addresses (slower, requires additional API calls)")
	listCmd.Flags().BoolP("detailed", "d", false, "Show detailed disk information (slower, requires additional API calls)")
	listCmd.Flags().Bool("json", false, "Output as JSON")
	vmCmd.AddCommand(listCmd)
}
