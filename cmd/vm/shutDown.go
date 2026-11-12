package cmd

import (
	"context"
	"os"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/cmdutil"
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
  prox vm stop myvm --force    # stop immediately, for guests that ignore ACPI shutdown
  prox vm shutdown --tag web   # Shut down every VM tagged 'web' (asks first)`,
	Args:              cmdutil.NameOrTagArgs,
	ValidArgsFunction: completion.FirstArgOnly(completion.GetVMNames),
	Run: func(cmd *cobra.Command, args []string) {
		wait := cmdutil.ResolveWait(cmd, false)
		force, _ := cmd.Flags().GetBool("force")
		node, _ := cmd.Flags().GetString("node")

		if tag, _ := cmd.Flags().GetString("tag"); tag != "" {
			verb := "shut down"
			if force {
				verb = "force stop"
			}
			err := runOnTaggedVMs(cmd, verb, func(ctx context.Context, cl c.ProxmoxClientInterface, v vm.VM) error {
				return vm.ShutdownVMWithOptions(ctx, v.ID, v.Node, wait, force, cl)
			})
			if err != nil {
				output.VMError("shutdown", err)
				os.Exit(1)
			}
			return
		}
		nameOrID := args[0]

		// Shutdown the VM
		if err := vm.ShutdownVMByNameOrIDWithOptions(nameOrID, node, wait, force); err != nil {
			output.VMError("shutdown", err)
			os.Exit(1)
		}
	},
}

func init() {
	cmdutil.AddWaitFlags(sdCmd, false)
	cmdutil.AddTagFlags(sdCmd, "VM")
	sdCmd.Flags().StringP("node", "n", "", "Only match VMs on this node (optional - auto-discovered if not specified)")
	sdCmd.Flags().BoolP("force", "f", false, "Stop immediately instead of an ACPI shutdown (like pulling the power)")
	vmCmd.AddCommand(sdCmd)
	completion.RegisterNodeFlag(sdCmd, "node")
}
