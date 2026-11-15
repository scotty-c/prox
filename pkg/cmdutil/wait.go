// Package cmdutil holds small helpers shared by prox's cobra commands.
package cmdutil

import (
	"context"
	"time"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

// WaitForTask waits for a Proxmox task to finish, showing a spinner while it
// runs. It stops waiting on Ctrl-C or after --timeout, leaving the task
// running on the server, and returns an error if the task did not succeed.
func WaitForTask(ctx context.Context, client c.TaskStatusGetter, node, taskID string) error {
	stop := output.StartSpinner(" Processing...")
	defer stop()
	return c.WaitForTaskInteractive(ctx, client, node, taskID)
}

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

// FinishTask is the ending every Proxmox task command shares. It prints the
// task ID. Without wait it points at 'prox logs --follow' and returns; with
// wait it waits for the task, then prints done and how long it took. A task
// that fails is returned as an error for the caller to describe.
func FinishTask(ctx context.Context, client c.TaskStatusGetter, node, taskID string, wait bool, done string) error {
	output.Result("Task started: %s\n", taskID)
	if !wait {
		output.Info("Tip: Follow progress with 'prox logs --follow %s'\n", taskID)
		return nil
	}
	start := time.Now()
	if err := WaitForTask(ctx, client, node, taskID); err != nil {
		return err
	}
	output.Result("%s in %s\n", done, time.Since(start).Round(time.Millisecond))
	return nil
}
