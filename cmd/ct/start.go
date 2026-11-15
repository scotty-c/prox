package cmd

import (
	"fmt"
	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/container"
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
  prox lxc start 101
  prox ct start --tag web    # Start every container tagged 'web' (asks first)`,
	Args:              cmdutil.NameOrTagArgs,
	ValidArgsFunction: completion.FirstArgOnly(completion.StoppedContainers),
	RunE: func(cmd *cobra.Command, args []string) error {
		wait := cmdutil.ResolveWait(cmd, true)

		if tag, _ := cmd.Flags().GetString("tag"); tag != "" {
			err := runOnTaggedContainers(cmd, "start", func(id string) error {
				return container.StartContainerWithWait(cmd.Context(), id, wait)
			})
			if err != nil {
				return fmt.Errorf("starting containers: %w", err)
			}
			return nil
		}
		nameOrID := args[0]

		// Start the container
		err := container.StartContainerWithWait(cmd.Context(), nameOrID, wait)
		if err != nil {
			return fmt.Errorf("starting container: %w", err)
		}
		return nil
	},
}

func init() {
	cmdutil.AddWaitFlags(startCmd, true)
	cmdutil.AddTagFlags(startCmd, "container")
	ctCmd.AddCommand(startCmd)
}
