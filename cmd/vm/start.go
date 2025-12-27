package cmd

import (
	"fmt"
	"os"

	"github.com/scotty-c/prox/pkg/vm"
	"github.com/spf13/cobra"
)

// startCmd represents the start command of a vm

var startCmd = &cobra.Command{
	Use:   "start <name|id>",
	Short: "Start a virtual machine",
	Long: `Start a virtual machine on the Proxmox VE server by providing the VM name or ID. The node will be automatically discovered if not specified.

Examples:
  prox vm start myvm
  prox vm start 100
  prox vm start web-server`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		nameOrID := args[0]

		// Start the VM
		if err := vm.StartVMByNameOrID(nameOrID); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	startCmd.Flags().StringP("node", "n", "", "Proxmox node name (optional - will be auto-discovered if not specified)")
	vmCmd.AddCommand(startCmd)
}
