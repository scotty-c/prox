package cmd

import (
	"os"

	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/container"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate <name|id> <target_node>",
	Short: "Move a container to another node",
	Long: `Move a container to another node in the cluster.

A running container can't be moved live. Use --restart to stop it, move it and
start it again on the target node.

Examples:
  prox ct migrate web node2              # Stopped container
  prox ct migrate web node2 --restart    # Running container`,
	Args: cobra.ExactArgs(2),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return completion.GetContainerNames(cmd, args, toComplete)
		case 1:
			return completion.GetNodeNames(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	Run: func(cmd *cobra.Command, args []string) {
		restart, _ := cmd.Flags().GetBool("restart")
		wait := cmdutil.ResolveWait(cmd, true)
		if err := container.MigrateContainer(args[0], args[1], restart, wait); err != nil {
			output.UserError("migrating container", err)
			os.Exit(1)
		}
	},
}

func init() {
	migrateCmd.Flags().Bool("restart", false, "Stop a running container, move it, and start it on the target node")
	cmdutil.AddWaitFlags(migrateCmd, true)
	ctCmd.AddCommand(migrateCmd)
}
