package cmd

import (
	"os"

	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/scotty-c/prox/pkg/vm"
	"github.com/spf13/cobra"
)

var rebootCmd = &cobra.Command{
	Use:     "reboot <name|id>",
	Aliases: []string{"restart"},
	Short:   "Reboot a running virtual machine",
	Long: `Reboot a running VM cleanly, via ACPI or the QEMU guest agent.

Examples:
  prox vm reboot web
  prox vm reboot 100 --wait`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completion.FirstArgOnly(completion.GetVMNames),
	Run: func(cmd *cobra.Command, args []string) {
		node, _ := cmd.Flags().GetString("node")
		wait := cmdutil.ResolveWait(cmd, false)
		if err := vm.RebootVMByNameOrID(args[0], node, wait); err != nil {
			output.VMError("reboot", err)
			os.Exit(1)
		}
	},
}

func init() {
	cmdutil.AddWaitFlags(rebootCmd, false)
	rebootCmd.Flags().StringP("node", "n", "", "Only match VMs on this node")
	vmCmd.AddCommand(rebootCmd)
	completion.RegisterNodeFlag(rebootCmd, "node")
}
