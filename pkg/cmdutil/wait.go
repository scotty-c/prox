// Package cmdutil holds small helpers shared by prox's cobra commands.
package cmdutil

import "github.com/spf13/cobra"

// AddWaitFlags registers --wait and --no-wait on cmd. defaultWait is what the
// command does when neither is given; it is shown in the help text.
func AddWaitFlags(cmd *cobra.Command, defaultWait bool) {
	waitHelp := "Wait for the operation to complete and show duration"
	noWaitHelp := "Return as soon as the task has started"
	if defaultWait {
		waitHelp += " (default)"
	} else {
		noWaitHelp += " (default)"
	}
	cmd.Flags().BoolP("wait", "w", false, waitHelp)
	cmd.Flags().Bool("no-wait", false, noWaitHelp)
}

// ResolveWait returns whether to wait, given the flags from AddWaitFlags.
// An explicit --wait or --no-wait overrides the default; --wait wins if both
// are given.
func ResolveWait(cmd *cobra.Command, defaultWait bool) bool {
	if cmd.Flags().Changed("wait") {
		wait, _ := cmd.Flags().GetBool("wait")
		return wait
	}
	if cmd.Flags().Changed("no-wait") {
		noWait, _ := cmd.Flags().GetBool("no-wait")
		return !noWait
	}
	return defaultWait
}
