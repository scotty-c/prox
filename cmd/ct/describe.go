package cmd

import (
	"fmt"
	"os"

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
	Run: func(cmd *cobra.Command, args []string) {
		nameOrID := args[0]

		jsonOutput, _ := cmd.Flags().GetBool("json")
		if jsonOutput {
			details, err := container.GetContainerDetails(nameOrID)
			if err != nil {
				output.UserError("getting container details", err)
				os.Exit(1)
			}
			if err := output.OutputJSON(details); err != nil {
				output.UserError("outputting JSON", err)
				os.Exit(1)
			}
			return
		}

		// Describe the container
		err := container.DescribeContainer(nameOrID)
		if err != nil {
			output.UserError("describing container", err)
			os.Exit(1)
		}
	},
}

func init() {
	describeCmd.Flags().Bool("json", false, "Output in JSON format")
	ctCmd.AddCommand(describeCmd)
}
