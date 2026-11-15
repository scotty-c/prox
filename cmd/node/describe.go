package node

import (
	"fmt"

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
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if cmdutil.StructuredOutput(cmd) {
			details, err := node.GetNodeDetails(name)
			if err != nil {
				return fmt.Errorf("getting node details: %w", err)
			}
			if err := output.OutputJSON(details); err != nil {
				return fmt.Errorf("outputting JSON: %w", err)
			}
			return nil
		}

		if err := node.DescribeNode(name); err != nil {
			return fmt.Errorf("describing node: %w", err)
		}
		return nil
	},
}

func init() {
	describeCmd.Flags().Bool("json", false, "Output in JSON format (same as -o json)")
	NodeCmd.AddCommand(describeCmd)
}
