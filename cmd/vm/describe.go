/*
Copyright © 2024 prox contributors
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/scotty-c/prox/pkg/vm"
	"github.com/spf13/cobra"
)

// describeCmd represents the describe command
var describeCmd = &cobra.Command{
	Use:     "describe <vm_name|vm_id>",
	Aliases: []string{"desc", "info"},
	Short:   "Show detailed information about a virtual machine",
	Long: `Show detailed information about a virtual machine including configuration, 
resource usage, network settings, and runtime status.

You can specify either the VM name or ID.

Examples:
  prox vm describe 100
  prox vm describe my-vm
  prox vm desc web-server
  prox vm info 101`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("requires exactly 1 argument: <vm_name|vm_id>")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Get VM name or ID from positional argument
		nameOrID := args[0]

		// Get node from flag (optional)
		node, _ := cmd.Flags().GetString("node")

		// Describe the VM
		err := vm.DescribeVM(nameOrID, node)
		if err != nil {
			fmt.Printf("Error: Error describing VM: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	describeCmd.Flags().StringP("node", "n", "", "Proxmox node name (optional - will be auto-discovered if not specified)")
	vmCmd.AddCommand(describeCmd)
}
