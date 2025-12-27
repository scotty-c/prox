package cmd

import (
	"fmt"
	"os"

	"github.com/scotty-c/prox/pkg/vm"
	"github.com/spf13/cobra"
)

// sdCmd represents the command to shutdown a vm

var sdCmd = &cobra.Command{
	Use:     "shutdown <name|id>",
	Aliases: []string{"stop"},
	Short:   "Shutdown a virtual machine",
	Long: `Shutdown a virtual machine on the Proxmox VE server by providing the VM name or ID. The node will be automatically discovered if not specified.

Examples:
  prox vm shutdown myvm
  prox vm stop 100
  prox vm shutdown web-server`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		nameOrID := args[0]

		// Shutdown the VM
		if err := vm.ShutdownVMByNameOrID(nameOrID); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	sdCmd.Flags().StringP("node", "n", "", "Proxmox node name (optional - will be auto-discovered if not specified)")
	vmCmd.AddCommand(sdCmd)
}
