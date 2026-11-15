package cmd

import (
	"context"
	"fmt"
	"os"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/cmdutil"

	"github.com/scotty-c/prox/pkg/completion"
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
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completion.FirstArgOnly(completion.GetVMNames),
	Run: func(cmd *cobra.Command, args []string) {
		wait := cmdutil.ResolveWait(cmd, false)
		nameOrID := args[0]

		// Get flags
		force, _ := cmd.Flags().GetBool("force")
		node, _ := cmd.Flags().GetString("node")

		ctx := context.Background()
		cl, err := c.CreateClient()
		if err != nil {
			output.ClientError(err)
			os.Exit(1)
		}
		// Look the VM up first so the question says exactly what will be deleted
		target, err := vm.FindByNameOrIDOnNode(ctx, cl, nameOrID, node)
		if err != nil {
			output.VMError("delete", err)
			os.Exit(1)
		}

		if !force && !cmdutil.ConfirmOrExit(
			fmt.Sprintf("Delete VM '%s' (ID %d, %s, on %s)? This action cannot be undone", target.Name, target.ID, target.Status, target.Node),
			"--force", "Deletion cancelled") {
			return
		}

		if err := vm.DeleteVMWithWait(ctx, target.ID, target.Node, wait); err != nil {
			output.VMError("delete", err)
			os.Exit(1)
		}
	},
}

func init() {
	cmdutil.AddWaitFlags(delCmd, false)
	delCmd.Flags().StringP("node", "n", "", "Only match VMs on this node (optional - auto-discovered if not specified)")
	delCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt (use with caution)")
	vmCmd.AddCommand(delCmd)
	completion.RegisterNodeFlag(delCmd, "node")
}
