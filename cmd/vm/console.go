package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/scotty-c/prox/pkg/vm"
	"github.com/spf13/cobra"
)

var consoleCmd = &cobra.Command{
	Use:   "console <name|id>",
	Short: "Attach to a VM's serial console",
	Long: `Attach to a running VM's serial console. This SSHes to the VM's Proxmox node
and runs 'qm terminal'. The VM needs a serial port (e.g. 'qm set <id> --serial0
socket') and a guest configured to use it, as most cloud images are. It needs
SSH access to the node (root by default). Press Ctrl-O to detach.

Examples:
  prox vm console web
  prox vm console 100 --host pve1.lan`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completion.FirstArgOnly(completion.GetVMNames),
	Run: func(cmd *cobra.Command, args []string) {
		node, _ := cmd.Flags().GetString("node")
		cl, err := c.CreateClient()
		if err != nil {
			output.ClientError(err)
			os.Exit(1)
		}
		v, err := vm.FindByNameOrIDOnNode(context.Background(), cl, args[0], node)
		if err != nil {
			output.VMError("console", err)
			os.Exit(1)
		}
		if v.Status != "running" {
			output.UserError("opening console", fmt.Errorf("VM %s is not running; start it with 'prox vm start %s'", v.Name, v.Name))
			os.Exit(1)
		}
		if err := cmdutil.RunOnNode(cmd, cl, v.Node, "qm", "terminal", strconv.Itoa(v.ID)); err != nil {
			output.UserError("opening console", err)
			os.Exit(1)
		}
	},
}

func init() {
	cmdutil.AddNodeSSHFlags(consoleCmd)
	consoleCmd.Flags().StringP("node", "n", "", "Only match VMs on this node")
	vmCmd.AddCommand(consoleCmd)
	completion.RegisterNodeFlag(consoleCmd, "node")
}
