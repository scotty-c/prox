package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/scotty-c/prox/pkg/vm"
	"github.com/spf13/cobra"
)

// editCmd edit vm's on the proxmox server

var editCmd = &cobra.Command{
	Use:   "edit [VM_ID] [flags]",
	Short: "Edit virtual machine configuration",
	Long:  `Edit virtual machine configuration such as name, CPU, and memory. The VM's node will be automatically discovered if not specified.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Parse VM ID from positional argument
		id, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Printf("Error: Invalid VM ID '%s'. Must be a number.\n", args[0])
			os.Exit(1)
		}

		// Get flags
		node, _ := cmd.Flags().GetString("node")
		name, _ := cmd.Flags().GetString("name")
		cpu, _ := cmd.Flags().GetInt("cpu")
		memory, _ := cmd.Flags().GetInt("memory")
		diskSize, _ := cmd.Flags().GetInt("disk")

		vm.EditVm(id, node, name, cpu, memory, diskSize)
	},
}

func init() {
	editCmd.Flags().StringP("node", "n", "", "Proxmox node name (optional - will be auto-discovered if not specified)")
	editCmd.Flags().StringP("name", "N", "", "VM name")
	editCmd.Flags().IntP("cpu", "c", 0, "Number of CPU cores")
	editCmd.Flags().IntP("memory", "m", 0, "Memory size in MB")
	editCmd.Flags().IntP("disk", "d", 0, "Disk size in GB")
	vmCmd.AddCommand(editCmd)
}
