package cmd

import (
	"fmt"
	"github.com/scotty-c/prox/pkg/cmdutil"
	"os"

	"github.com/scotty-c/prox/pkg/completion"
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
	ValidArgsFunction: completion.FirstArgOnly(completion.GetContainerNames),
	Run: func(cmd *cobra.Command, args []string) {
		wait := cmdutil.ResolveWait(cmd, true)
		nameOrID := args[0]

		// Start the container
		err := container.StartContainerWithWait(nameOrID, wait)
		if err != nil {
			output.UserError("starting container", err)
			os.Exit(1)
		}
	},
}

func init() {
	cmdutil.AddWaitFlags(startCmd, true)
	ctCmd.AddCommand(startCmd)
}
