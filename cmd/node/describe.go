package node

import (
	"fmt"
	"os"

	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/node"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

var describeCmd = &cobra.Command{
	Use:               "describe <node_name>",
	Aliases:           []string{"info"},
	Short:             "Show detailed information about a Proxmox node",
	ValidArgsFunction: completion.FirstArgOnly(completion.GetNodeNames),
	Args:              cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		if cmdutil.StructuredOutput(cmd) {
			details, err := node.GetNodeDetails(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting node details: %v\n", err)
				os.Exit(1)
			}
			if err := output.OutputJSON(details); err != nil {
				fmt.Fprintf(os.Stderr, "Error outputting JSON: %v\n", err)
				os.Exit(1)
			}
			return
		}

		if err := node.DescribeNode(name); err != nil {
			fmt.Fprintf(os.Stderr, "Error describing node: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	describeCmd.Flags().Bool("json", false, "Output in JSON format (same as -o json)")
	NodeCmd.AddCommand(describeCmd)
}
