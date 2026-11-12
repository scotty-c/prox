package cmdutil

import (
	"context"
	"fmt"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

// AddNodeSSHFlags registers the flags for commands that SSH to a Proxmox node
func AddNodeSSHFlags(cmd *cobra.Command) {
	cmd.Flags().String("host", "", "Address to SSH to for the node (default: the node's cluster address)")
	cmd.Flags().StringP("user", "u", "root", "SSH user on the Proxmox node")
	cmd.Flags().IntP("port", "p", 22, "SSH port on the Proxmox node")
	cmd.Flags().Bool("print", false, "Print the ssh command instead of running it")
}

// RunOnNode runs remote interactively on node over SSH, using the flags from
// AddNodeSSHFlags
func RunOnNode(cmd *cobra.Command, cl c.ProxmoxClientInterface, node string, remote ...string) error {
	host, _ := cmd.Flags().GetString("host")
	user, _ := cmd.Flags().GetString("user")
	port, _ := cmd.Flags().GetInt("port")
	printOnly, _ := cmd.Flags().GetBool("print")

	if host == "" {
		var err error
		if host, err = cl.GetNodeAddress(context.Background(), node); err != nil {
			return fmt.Errorf("couldn't find an address for node %s (use --host): %w", node, err)
		}
	}

	args := NodeSSHArgs(host, user, port, remote...)
	if printOnly {
		fmt.Print("ssh")
		for _, a := range args {
			fmt.Print(" ", a)
		}
		fmt.Println()
		return nil
	}
	output.Error("Connecting to node %s (%s)...\n", node, host)
	return RunSSH(args)
}
