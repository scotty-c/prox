package cmd

import (
	"os"

	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/scotty-c/prox/pkg/vm"
	"github.com/spf13/cobra"
)

// editCmd edit vm's on the proxmox server

var editCmd = &cobra.Command{
	Use:   "edit <name|id> [flags]",
	Short: "Edit virtual machine configuration",
	Long: `Edit virtual machine configuration such as name, CPU, and memory. The VM's node will be automatically discovered if not specified.

Examples:
  prox vm edit myvm --name newname
  prox vm edit 100 --cpu 4 --memory 4096
  prox vm edit web-server --disk 50`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completion.GetVMNames,
	Run: func(cmd *cobra.Command, args []string) {
		nameOrID := args[0]

		// Get flags
		name, _ := cmd.Flags().GetString("name")
		cpu, _ := cmd.Flags().GetInt("cpu")
		memory, _ := cmd.Flags().GetInt("memory")
		diskSize, _ := cmd.Flags().GetInt("disk")

		// Edit the VM
		if err := vm.EditVMByNameOrID(nameOrID, name, cpu, memory, diskSize); err != nil {
			output.VMError("edit", err)
			os.Exit(1)
		}
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
