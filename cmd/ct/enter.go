package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/container"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

var enterCmd = &cobra.Command{
	Use:     "enter <name|id>",
	Aliases: []string{"exec", "shell"},
	Short:   "Open a shell inside a container",
	Long: `Open a root shell inside a running container. This SSHes to the container's
Proxmox node and runs 'pct enter', so it works even when the container has no
network or SSH server. It needs SSH access to the node (root by default).

Examples:
  prox ct enter web
  prox ct enter 101 --host pve1.lan
  prox ct enter web --print     # Show the ssh command instead`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completion.FirstArgOnly(completion.GetContainerNames),
	Run: func(cmd *cobra.Command, args []string) {
		cl, err := c.CreateClient()
		if err != nil {
			output.ClientError(err)
			os.Exit(1)
		}
		ct, err := container.FindByNameOrID(context.Background(), cl, args[0])
		if err != nil {
			output.ContainerError("enter", err)
			os.Exit(1)
		}
		if ct.Status != "running" {
			output.UserError("entering container", fmt.Errorf("container %s is not running; start it with 'prox ct start %s'", ct.Name, ct.Name))
			os.Exit(1)
		}
		if err := cmdutil.RunOnNode(cmd, cl, ct.Node, "pct", "enter", strconv.Itoa(ct.ID)); err != nil {
			output.UserError("entering container", err)
			os.Exit(1)
		}
	},
}

func init() {
	cmdutil.AddNodeSSHFlags(enterCmd)
	ctCmd.AddCommand(enterCmd)
}
