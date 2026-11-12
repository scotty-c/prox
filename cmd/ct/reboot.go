package cmd

import (
	"os"

	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/container"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

var rebootCmd = &cobra.Command{
	Use:               "reboot <name|id>",
	Aliases:           []string{"restart"},
	Short:             "Reboot a running container",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completion.FirstArgOnly(completion.GetContainerNames),
	Run: func(cmd *cobra.Command, args []string) {
		wait := cmdutil.ResolveWait(cmd, true)
		if err := container.RebootContainer(args[0], wait); err != nil {
			output.UserError("rebooting container", err)
			os.Exit(1)
		}
	},
}

func init() {
	cmdutil.AddWaitFlags(rebootCmd, true)
	ctCmd.AddCommand(rebootCmd)
}
