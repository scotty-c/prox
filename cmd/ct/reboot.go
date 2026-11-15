package cmd

import (
	"fmt"
	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/container"
	"github.com/spf13/cobra"
)

var rebootCmd = &cobra.Command{
	Use:               "reboot <name|id>",
	Aliases:           []string{"restart"},
	Short:             "Reboot a running container",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completion.FirstArgOnly(completion.GetContainerNames),
	RunE: func(cmd *cobra.Command, args []string) error {
		wait := cmdutil.ResolveWait(cmd, true)
		if err := container.RebootContainer(args[0], wait); err != nil {
			return fmt.Errorf("rebooting container: %w", err)
		}
		return nil
	},
}

func init() {
	cmdutil.AddWaitFlags(rebootCmd, true)
	ctCmd.AddCommand(rebootCmd)
}
