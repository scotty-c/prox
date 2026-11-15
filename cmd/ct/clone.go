package cmd

import (
	"fmt"

	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/container"
	"github.com/spf13/cobra"
)

var cloneCmd = &cobra.Command{
	Use:   "clone <source_name|id> [new_id] [name]",
	Short: "Clone a container",
	Long: `Clone an existing container to a new container on the same node. Without a new
ID the next free one is used.

A linked clone is only possible when the source is a template; use --full to copy
an ordinary container. The name can be given as a third argument or with --name.

Examples:
  prox ct clone web web-copy --full       # next free ID
  prox ct clone web 201 web-copy --full
  prox ct clone ubuntu-template 202 api`,
	Args:              cobra.RangeArgs(1, 3),
	ValidArgsFunction: completion.FirstArgOnly(completion.GetContainerNames),
	RunE: func(cmd *cobra.Command, args []string) error {
		newID, name, err := cmdutil.ParseCloneArgs(args[1:])
		if err != nil {
			return err
		}
		if name == "" {
			name, _ = cmd.Flags().GetString("name")
		}
		full, _ := cmd.Flags().GetBool("full")
		wait := cmdutil.ResolveWait(cmd, true)

		if err := container.CloneContainer(args[0], newID, name, full, wait); err != nil {
			return fmt.Errorf("cloning container: %w", err)
		}
		return nil
	},
}

func init() {
	cloneCmd.Flags().StringP("name", "N", "", "Hostname for the new container (alternative to the third argument)")
	cloneCmd.Flags().BoolP("full", "f", false, "Create a full copy instead of a linked clone (required unless the source is a template)")
	cmdutil.AddWaitFlags(cloneCmd, true)
	ctCmd.AddCommand(cloneCmd)
}
