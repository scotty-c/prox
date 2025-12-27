package cmd

import (
	"fmt"
	"os"

	"github.com/scotty-c/prox/pkg/output"
	"github.com/scotty-c/prox/pkg/vm"
	"github.com/spf13/cobra"
)

// delCmd represents delete command

var delCmd = &cobra.Command{
	Use:     "delete <name|id>",
	Aliases: []string{"del", "rm"},
	Short:   "Delete a virtual machine",
	Long: `Delete a virtual machine from the Proxmox VE server by providing the VM name or ID. The node will be automatically discovered if not specified.

Examples:
  prox vm delete myvm
  prox vm rm 100
  prox vm delete web-server --force`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		nameOrID := args[0]

		// Get flags
		force, _ := cmd.Flags().GetBool("force")

		// Confirm deletion unless --force is used
		if !force {
			if !output.Confirm(fmt.Sprintf("Are you sure you want to delete VM '%s'? This action cannot be undone", nameOrID)) {
				fmt.Println("Deletion cancelled")
				return
			}
		}

		// Delete the VM
		if err := vm.DeleteVMByNameOrID(nameOrID); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	delCmd.Flags().StringP("node", "n", "", "Proxmox node name (optional - will be auto-discovered if not specified)")
	delCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt (use with caution)")
	vmCmd.AddCommand(delCmd)
}
