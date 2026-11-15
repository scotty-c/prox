package cmd

import (
	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/tasks"
	"github.com/spf13/cobra"
)

var tasksCmd = &cobra.Command{
	Use:     "tasks",
	GroupID: "utilities",
	Short:   "List recent Proxmox tasks",
	Long: `List the cluster's recent tasks, newest first: what ran, on which node, who
started it, how long it took and whether it succeeded. Use it to find the task
ID of an operation, then follow it with 'prox logs'.

Examples:
  prox tasks                    # the 20 most recent
  prox tasks --running          # what is still running
  prox tasks --failed -n pve1   # failures on one node
  prox tasks --upid --limit 5   # include task IDs for 'prox logs'
  prox tasks -o json            # for scripts (includes task IDs)`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := tasks.Options{}
		opts.Limit, _ = cmd.Flags().GetInt("limit")
		opts.Node, _ = cmd.Flags().GetString("node")
		opts.Running, _ = cmd.Flags().GetBool("running")
		opts.Failed, _ = cmd.Flags().GetBool("failed")
		opts.ShowUPID, _ = cmd.Flags().GetBool("upid")
		return tasks.List(cmd.Context(), opts)
	},
}

func init() {
	tasksCmd.Flags().Int("limit", 20, "Show at most this many tasks (0 = all)")
	tasksCmd.Flags().StringP("node", "n", "", "Only tasks on this node")
	tasksCmd.Flags().Bool("running", false, "Only tasks that are still running")
	tasksCmd.Flags().Bool("failed", false, "Only tasks that failed")
	tasksCmd.Flags().Bool("upid", false, "Include the task ID, for 'prox logs'")
	RootCmd.AddCommand(tasksCmd)
	completion.RegisterNodeFlag(tasksCmd, "node")
}
