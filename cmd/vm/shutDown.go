package cmd

import (
	"os"

	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/output"
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
  prox vm shutdown web-server
  prox vm stop myvm --force    # stop immediately, for guests that ignore ACPI shutdown`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completion.FirstArgOnly(completion.GetVMNames),
	Run: func(cmd *cobra.Command, args []string) {
		nameOrID := args[0]
		wait, _ := cmd.Flags().GetBool("wait")
		force, _ := cmd.Flags().GetBool("force")
		node, _ := cmd.Flags().GetString("node")

		// Shutdown the VM
		if err := vm.ShutdownVMByNameOrIDWithOptions(nameOrID, node, wait, force); err != nil {
			output.VMError("shutdown", err)
			os.Exit(1)
		}
	},
}

func init() {
	sdCmd.Flags().StringP("node", "n", "", "Only match VMs on this node (optional - auto-discovered if not specified)")
	sdCmd.Flags().BoolP("force", "f", false, "Stop immediately instead of an ACPI shutdown (like pulling the power)")
	sdCmd.Flags().BoolP("wait", "w", false, "Wait for the operation to complete and show duration")
	vmCmd.AddCommand(sdCmd)
	completion.RegisterNodeFlag(sdCmd, "node")
}
