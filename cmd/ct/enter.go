package cmd

import (
	"context"
	"fmt"
	"strconv"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/container"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := c.CreateClient()
		if err != nil {
			return err
		}
		ct, err := container.FindByNameOrID(context.Background(), cl, args[0])
		if err != nil {
			return err
		}
		if ct.Status != "running" {
			return fmt.Errorf("entering container: container %s is not running; start it with 'prox ct start %s'", ct.Name, ct.Name)
		}
		if err := cmdutil.RunOnNode(cmd, cl, ct.Node, "pct", "enter", strconv.Itoa(ct.ID)); err != nil {
			return fmt.Errorf("entering container: %w", err)
		}
		return nil
	},
}

func init() {
	cmdutil.AddNodeSSHFlags(enterCmd)
	ctCmd.AddCommand(enterCmd)
}
