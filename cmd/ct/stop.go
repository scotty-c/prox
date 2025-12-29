package cmd

import (
	"fmt"
	"os"

	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/container"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:     "stop <name|id>",
	Aliases: []string{"shutdown"},
	Short:   "Stop an LXC container",
	Long: `Stop an LXC container by name or ID.

Examples:
  prox ct stop mycontainer
  prox ct stop 100
  prox container shutdown webapp
  prox lxc stop 101`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("requires exactly 1 argument: <name|id>")
		}
		return nil
	},
	ValidArgsFunction: completion.GetContainerNames,
	Run: func(cmd *cobra.Command, args []string) {
		nameOrID := args[0]

		// Stop the container
		err := container.StopContainer(nameOrID)
		if err != nil {
			output.UserError("stopping container", err)
			os.Exit(1)
		}
	},
}

func init() {
	ctCmd.AddCommand(stopCmd)
}
