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

// TaskResult is what a command that starts a Proxmox task prints with
// --output json or yaml
type TaskResult struct {
	Task    string  `json:"task"`              // the task ID (UPID), for 'prox logs'
	Node    string  `json:"node"`              // the node the task runs on
	Waited  bool    `json:"waited"`            // whether prox waited for it to finish
	Status  string  `json:"status"`            // "started" if not waited for, "ok" if it finished successfully
	Message string  `json:"message,omitempty"` // what was done, once it finished
	Seconds float64 `json:"seconds,omitempty"` // how long the wait took
}

// FinishTask is the ending every Proxmox task command shares. Without wait
// it reports the task and returns; with wait it waits for the task, then
// reports done and how long it took. A task that fails is returned as an
// error for the caller to describe.
//
// As text it prints the task ID and a result line (with a pointer to
// 'prox logs --follow' when not waiting). With --output json or yaml it prints
// one TaskResult instead.
//
// Inside RunBulk, a command that waits does not wait here: it reports the task
// as started and leaves it for RunBulk to wait for together with the others.
func FinishTask(ctx context.Context, client c.TaskStatusGetter, node, taskID string, wait bool, done string) error {
	structured := output.Structured()
	if !structured {
		output.Result("Task started: %s\n", taskID)
	}
	if b := batchFrom(ctx); b != nil && wait {
		b.tasks = append(b.tasks, pendingTask{node: node, id: taskID, done: done, started: time.Now()})
		return nil
	}
	if !wait {
		if structured {
			return output.OutputJSON(TaskResult{Task: taskID, Node: node, Status: "started"})
		}
		output.Info("Tip: Follow progress with 'prox logs --follow %s'\n", taskID)
		return nil
	}
	start := time.Now()
	if err := WaitForTask(ctx, client, node, taskID); err != nil {
		return err
	}
	took := time.Since(start)
	if structured {
		return output.OutputJSON(TaskResult{Task: taskID, Node: node, Waited: true, Status: "ok", Message: done, Seconds: took.Seconds()})
	}
	output.Result("%s in %s\n", done, took.Round(time.Millisecond))
	return nil
}
