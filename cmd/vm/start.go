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

// startCmd represents the start command of a vm

var startCmd = &cobra.Command{
	Use:   "start <name|id>",
	Short: "Start a virtual machine",
	Long: `Start a virtual machine on the Proxmox VE server by providing the VM name or ID. The node will be automatically discovered if not specified.

Examples:
  prox vm start myvm
  prox vm start 100
  prox vm start web-server
  prox vm start --tag web             # Start every VM tagged 'web' (asks first)`,
	Args:              cmdutil.NameOrTagArgs,
	ValidArgsFunction: completion.FirstArgOnly(completion.GetVMNames),
	Run: func(cmd *cobra.Command, args []string) {
		wait := cmdutil.ResolveWait(cmd, false)
		node, _ := cmd.Flags().GetString("node")

		if tag, _ := cmd.Flags().GetString("tag"); tag != "" {
			err := runOnTaggedVMs(cmd, "start", func(ctx context.Context, cl c.ProxmoxClientInterface, v vm.VM) error {
				return vm.StartVMWithWait(ctx, v.ID, v.Node, wait, cl)
			})
			if err != nil {
				output.VMError("start", err)
				os.Exit(1)
			}
			return
		}
		nameOrID := args[0]

		// Start the VM
		if err := vm.StartVMByNameOrIDWithWait(nameOrID, node, wait); err != nil {
			output.VMError("start", err)
			os.Exit(1)
		}
	},
}

func init() {
	cmdutil.AddWaitFlags(startCmd, false)
	cmdutil.AddTagFlags(startCmd, "VM")
	startCmd.Flags().StringP("node", "n", "", "Only match VMs on this node (optional - auto-discovered if not specified)")
	vmCmd.AddCommand(startCmd)
	completion.RegisterNodeFlag(startCmd, "node")
}
