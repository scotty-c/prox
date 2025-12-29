/*
Copyright © 2024 prox contributors
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/output"
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
  prox vm describe my-vm --json
  prox vm desc web-server
  prox vm info 101`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("requires exactly 1 argument: <vm_name|vm_id>")
		}
		return nil
	},
	ValidArgsFunction: completion.GetVMNames,
	Run: func(cmd *cobra.Command, args []string) {
		// Get VM name or ID from positional argument
		nameOrID := args[0]

		// Get flags
		node, _ := cmd.Flags().GetString("node")
		jsonOutput, _ := cmd.Flags().GetBool("json")

		if jsonOutput {
			details, err := vm.GetVMDetails(nameOrID, node)
			if err != nil {
				output.UserError("getting VM details", err)
				os.Exit(1)
			}
			if err := output.OutputJSON(details); err != nil {
				output.UserError("outputting JSON", err)
				os.Exit(1)
			}
			return
		}

		// Describe the VM
		err := vm.DescribeVM(nameOrID, node)
		if err != nil {
			output.UserError("describing VM", err)
			os.Exit(1)
		}
	},
}

func init() {
	describeCmd.Flags().StringP("node", "n", "", "Proxmox node name (optional - will be auto-discovered if not specified)")
	describeCmd.Flags().Bool("json", false, "Output in JSON format")
	vmCmd.AddCommand(describeCmd)
}
