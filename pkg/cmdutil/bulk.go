package cmdutil

import (
	"context"
	"fmt"
	"time"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/scotty-c/prox/pkg/util"
	"github.com/spf13/cobra"
)

// AddTagFlags registers --tag (act on every resource with a tag instead of a
// single name) and -y/--yes (skip the confirmation for --tag)
func AddTagFlags(cmd *cobra.Command, kind string) {
	cmd.Flags().StringP("tag", "t", "", fmt.Sprintf("Act on every %s with this tag instead of a single name", kind))
	cmd.Flags().BoolP("yes", "y", false, "With --tag, don't ask for confirmation")
}

// NameOrTagArgs accepts exactly one name, or none when --tag is given
func NameOrTagArgs(cmd *cobra.Command, args []string) error {
	tag, _ := cmd.Flags().GetString("tag")
	switch {
	case tag != "" && len(args) > 0:
		return fmt.Errorf("give either a name or --tag, not both")
	case tag == "" && len(args) != 1:
		return fmt.Errorf("requires exactly 1 argument: <name|id> (or use --tag)")
	}
	return nil
}

// BulkTarget is one resource in a bulk operation
type BulkTarget struct {
	Label string // e.g. "web-1 (ID: 101) on pve1"
	// Run does the operation. It must use the context it is given, which is
	// how RunBulk collects the tasks the operation starts.
	Run func(ctx context.Context) error
}

// pendingTask is a task a target started that RunBulk has yet to wait for
type pendingTask struct {
	node, id, done string
	started        time.Time
}

// batch holds the tasks one target started. A target runs on one goroutine at
// a time, so it needs no lock.
type batch struct{ tasks []pendingTask }

type batchKey struct{}

func withBatch(ctx context.Context) (context.Context, *batch) {
	b := &batch{}
	return context.WithValue(ctx, batchKey{}, b), b
}

func batchFrom(ctx context.Context) *batch {
	b, _ := ctx.Value(batchKey{}).(*batch)
	return b
}

// RunBulk lists the targets, asks for confirmation unless skipConfirm, then
// runs them. It keeps going after a failure and returns an error if any
// failed. verb describes the action, e.g. "start".
//
// With wait, it starts every target first, one after another, and then waits
// for all their tasks at once, so ten VMs take about as long as the slowest
// rather than the sum. The results are printed in target order once all have
// finished. Without wait each target runs to the point its task has started.
func RunBulk(ctx context.Context, client c.TaskStatusGetter, verb, kind, tag string, targets []BulkTarget, skipConfirm, wait bool) error {
	if len(targets) == 0 {
		return fmt.Errorf("no %ss have the tag '%s'", kind, tag)
	}

	output.Error("This will %s %d %s(s) tagged '%s':\n", verb, len(targets), kind, tag)
	for _, t := range targets {
		output.Error("  • %s\n", t.Label)
	}
	if !skipConfirm {
		ok, err := output.Confirm("Continue?", "--yes")
		if err != nil {
			return err
		}
		if !ok {
			output.Errorln("Cancelled")
			return nil
		}
	}

	// Phase 1: start each target. When waiting, its tasks are collected
	// instead of waited for.
	failed := 0
	batches := make([]*batch, len(targets))
	for i, t := range targets {
		tctx := ctx
		if wait {
			tctx, batches[i] = withBatch(ctx)
		}
		if err := t.Run(tctx); err != nil {
			// The loop carries on, so say which target failed as it happens
			output.Error("Error: %s: %v\n", t.Label, err)
			failed++
			batches[i] = nil
		}
	}

	if wait {
		failed += waitForBatches(ctx, client, verb, targets, batches)
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d %s(s) failed to %s", failed, len(targets), kind, verb)
	}
	return nil
}

// waitForBatches waits for every collected task at once, then reports each
// target in order. It returns how many targets had a task that failed.
func waitForBatches(ctx context.Context, client c.TaskStatusGetter, verb string, targets []BulkTarget, batches []*batch) int {
	type outcome struct {
		err  error
		took time.Duration
	}
	type job struct{ target, task int }
	var jobs []job
	outcomes := make([][]outcome, len(batches))
	for i, b := range batches {
		if b == nil {
			continue
		}
		outcomes[i] = make([]outcome, len(b.tasks))
		for k := range b.tasks {
			jobs = append(jobs, job{i, k})
		}
	}
	if len(jobs) == 0 {
		return 0
	}

	stop := output.StartSpinner(fmt.Sprintf(" Waiting for %d task(s)...", len(jobs)))
	util.ForEach(len(jobs), c.MaxConcurrentLookups, func(j int) {
		t := batches[jobs[j].target].tasks[jobs[j].task]
		err := c.WaitForTaskInteractive(ctx, client, t.node, t.id)
		outcomes[jobs[j].target][jobs[j].task] = outcome{err, time.Since(t.started)}
	})
	stop()

	failed := 0
	for i, b := range batches {
		if b == nil {
			continue
		}
		bad := false
		for k, t := range b.tasks {
			o := outcomes[i][k]
			switch {
			case o.err != nil:
				output.Error("Error: %s: %s failed: %v\n", targets[i].Label, verb, o.err)
				bad = true
			case output.Structured():
				_ = output.OutputJSON(TaskResult{Task: t.id, Node: t.node, Waited: true, Status: "ok", Message: t.done, Seconds: o.took.Seconds()})
			default:
				output.Result("%s in %s\n", t.done, o.took.Round(time.Millisecond))
			}
		}
		if bad {
			failed++
		}
	}
	return failed
}
