package cmd

import (
	"fmt"

	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/container"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

// describeCmd represents the describe command
var describeCmd = &cobra.Command{
	Use:     "describe <name|id>",
	Aliases: []string{"desc", "info"},
	Short:   "Show detailed information about an LXC container",
	Long: `Show detailed information about an LXC container including configuration, 
resource usage, network settings, and runtime status.

Examples:
  prox ct describe mycontainer
  prox ct describe 100 --json
  prox container desc webapp
  prox lxc info 101`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("requires exactly 1 argument: <name|id>")
		}
		return nil
	},
	ValidArgsFunction: completion.FirstArgOnly(completion.GetContainerNames),
	RunE: func(cmd *cobra.Command, args []string) error {
		nameOrID := args[0]

		if cmdutil.StructuredOutput(cmd) {
			details, err := container.GetContainerDetails(cmd.Context(), nameOrID)
			if err != nil {
				return fmt.Errorf("getting container details: %w", err)
			}
			if err := output.OutputJSON(details); err != nil {
				return fmt.Errorf("outputting JSON: %w", err)
			}
			return nil
		}

		// Describe the container
		err := container.DescribeContainer(cmd.Context(), nameOrID)
		if err != nil {
			return fmt.Errorf("describing container: %w", err)
		}
		return nil
	},
}

func init() {
	describeCmd.Flags().Bool("json", false, "Output in JSON format (same as -o json)")
	ctCmd.AddCommand(describeCmd)
}
