package node

import (
	"fmt"
	"os"

	"github.com/scotty-c/prox/pkg/node"
	"github.com/spf13/cobra"
)

var describeCmd = &cobra.Command{
	Use:     "describe <node_name>",
	Aliases: []string{"info"},
	Short:   "Show detailed information about a Proxmox node",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		if err := node.DescribeNode(name); err != nil {
			fmt.Fprintf(os.Stderr, "Error describing node: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	NodeCmd.AddCommand(describeCmd)
}
