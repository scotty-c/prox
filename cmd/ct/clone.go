package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/container"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

var cloneCmd = &cobra.Command{
	Use:   "clone <source_name|id> <new_id> [name]",
	Short: "Clone a container",
	Long: `Clone an existing container to a new container ID on the same node.

A linked clone is only possible when the source is a template; use --full to copy
an ordinary container. The name can be given as a third argument or with --name.

Examples:
  prox ct clone web 201 web-copy --full
  prox ct clone ubuntu-template 202 api`,
	Args:              cobra.RangeArgs(2, 3),
	ValidArgsFunction: completion.FirstArgOnly(completion.GetContainerNames),
	Run: func(cmd *cobra.Command, args []string) {
		newID, err := strconv.Atoi(args[1])
		if err != nil || newID <= 0 {
			fmt.Fprintf(os.Stderr, "Error: Invalid new container ID '%s'. Must be a positive number.\n", args[1])
			os.Exit(1)
		}

		name, _ := cmd.Flags().GetString("name")
		if len(args) == 3 {
			name = args[2]
		}
		full, _ := cmd.Flags().GetBool("full")
		wait := cmdutil.ResolveWait(cmd, true)

		if err := container.CloneContainer(args[0], newID, name, full, wait); err != nil {
			output.UserError("cloning container", err)
			os.Exit(1)
		}
	},
}

func init() {
	cloneCmd.Flags().StringP("name", "N", "", "Hostname for the new container (alternative to the third argument)")
	cloneCmd.Flags().BoolP("full", "f", false, "Create a full copy instead of a linked clone (required unless the source is a template)")
	cmdutil.AddWaitFlags(cloneCmd, true)
	ctCmd.AddCommand(cloneCmd)
}
