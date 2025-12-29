package cmd

import (
	"fmt"
	"os"

	"github.com/scotty-c/prox/pkg/container"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start <name|id>",
	Short: "Start an LXC container",
	Long: `Start an LXC container by name or ID.

Examples:
  prox ct start mycontainer
  prox ct start 100
  prox container start webapp
  prox lxc start 101`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("requires exactly 1 argument: <name|id>")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		nameOrID := args[0]

		// Start the container
		err := container.StartContainer(nameOrID)
		if err != nil {
			output.UserError("starting container", err)
			os.Exit(1)
		}
	},
}

func init() {
	ctCmd.AddCommand(startCmd)
}
