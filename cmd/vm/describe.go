/*
Copyright © 2024 prox contributors
*/
package cmd

import (
	"fmt"

	"github.com/scotty-c/prox/pkg/cmdutil"
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
	ValidArgsFunction: completion.FirstArgOnly(completion.GetVMNames),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get VM name or ID from positional argument
		nameOrID := args[0]

		// Get flags
		node, _ := cmd.Flags().GetString("node")
		if cmdutil.StructuredOutput(cmd) {
			details, err := vm.GetVMDetails(nameOrID, node)
			if err != nil {
				return fmt.Errorf("getting VM details: %w", err)
			}
			if err := output.OutputJSON(details); err != nil {
				return fmt.Errorf("outputting JSON: %w", err)
			}
			return nil
		}

		// Describe the VM
		err := vm.DescribeVM(nameOrID, node)
		if err != nil {
			return fmt.Errorf("describing VM: %w", err)
		}
		return nil
	},
}

func init() {
	describeCmd.Flags().StringP("node", "n", "", "Proxmox node name (optional - will be auto-discovered if not specified)")
	describeCmd.Flags().Bool("json", false, "Output in JSON format (same as -o json)")
	vmCmd.AddCommand(describeCmd)
	completion.RegisterNodeFlag(describeCmd, "node")
}
