package cmd

import (
	"github.com/scotty-c/prox/pkg/cmdutil"
	"os"

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
  prox vm start web-server`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completion.FirstArgOnly(completion.GetVMNames),
	Run: func(cmd *cobra.Command, args []string) {
		nameOrID := args[0]
		wait := cmdutil.ResolveWait(cmd, false)
		node, _ := cmd.Flags().GetString("node")

		// Start the VM
		if err := vm.StartVMByNameOrIDWithWait(nameOrID, node, wait); err != nil {
			output.VMError("start", err)
			os.Exit(1)
		}
	},
}

func init() {
	cmdutil.AddWaitFlags(startCmd, false)
	startCmd.Flags().StringP("node", "n", "", "Only match VMs on this node (optional - auto-discovered if not specified)")
	vmCmd.AddCommand(startCmd)
	completion.RegisterNodeFlag(startCmd, "node")
}
