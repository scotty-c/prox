package cmd

import (
	"os"

	"github.com/scotty-c/prox/pkg/cmdutil"
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
  prox lxc stop 101
  prox ct stop mycontainer --force   # stop immediately
  prox ct stop --tag web             # Stop every container tagged 'web' (asks first)`,
	Args:              cmdutil.NameOrTagArgs,
	ValidArgsFunction: completion.FirstArgOnly(completion.GetContainerNames),
	Run: func(cmd *cobra.Command, args []string) {
		wait := cmdutil.ResolveWait(cmd, true)
		force, _ := cmd.Flags().GetBool("force")

		if tag, _ := cmd.Flags().GetString("tag"); tag != "" {
			verb := "stop"
			if force {
				verb = "force stop"
			}
			err := runOnTaggedContainers(cmd, verb, func(id string) error {
				return container.StopContainerWithOptions(id, force, wait)
			})
			if err != nil {
				output.UserError("stopping containers", err)
				os.Exit(1)
			}
			return
		}
		nameOrID := args[0]

		// Stop the container
		err := container.StopContainerWithOptions(nameOrID, force, wait)
		if err != nil {
			output.UserError("stopping container", err)
			os.Exit(1)
		}
	},
}

func init() {
	cmdutil.AddWaitFlags(stopCmd, true)
	cmdutil.AddTagFlags(stopCmd, "container")
	stopCmd.Flags().BoolP("force", "f", false, "Stop immediately instead of shutting down cleanly (like pulling the power)")
	ctCmd.AddCommand(stopCmd)
}
