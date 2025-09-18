package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/scotty-c/prox/pkg/vm"
	"github.com/spf13/cobra"
)

// delCmd represents delete command

var delCmd = &cobra.Command{
	Use:     "delete [VM_ID] [flags]",
	Aliases: []string{"del", "rm"},
	Short:   "Delete a virtual machine",
	Long:    `Delete a virtual machine from the Proxmox VE server by providing the VM ID. The node will be automatically discovered if not specified.`,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Parse VM ID from positional argument
		id, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Printf("Error: Invalid VM ID '%s'. Must be a number.\n", args[0])
			os.Exit(1)
		}

		// Get node from flag (optional)
		node, _ := cmd.Flags().GetString("node")

		vm.DeleteVm(id, node)
	},
}

func init() {
	delCmd.Flags().StringP("node", "n", "", "Proxmox node name (optional - will be auto-discovered if not specified)")
	vmCmd.AddCommand(delCmd)
}
