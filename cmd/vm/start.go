package cmd

import (
	"fmt"
	"os"

	"github.com/scotty-c/prox/pkg/vm"
	"github.com/spf13/cobra"
)

// startCmd represents the start command of a vm

var startCmd = &cobra.Command{
	Use:   "start [VM_ID] [flags]",
	Short: "Start a virtual machine",
	Long:  `Start a virtual machine on the Proxmox VE server by providing the VM ID. The node will be automatically discovered if not specified.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Get VM ID from positional argument
		vmID := args[0]

		// Convert string to int
		id := 0
		if _, err := fmt.Sscanf(vmID, "%d", &id); err != nil {
			fmt.Printf("Error: Invalid VM ID '%s'. Must be a number.\n", vmID)
			os.Exit(1)
		}

		// Get node from flag (optional)
		node, _ := cmd.Flags().GetString("node")

		vm.StartVm(id, node)
	},
}

func init() {
	startCmd.Flags().StringP("node", "n", "", "Proxmox node name (optional - will be auto-discovered if not specified)")
	vmCmd.AddCommand(startCmd)
}
