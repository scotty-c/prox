package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"
)

// Helper functions for common Proxmox operations
// These complement the existing client.go functionality

// TaskStatusGetter is the subset of the client needed to poll a task
type TaskStatusGetter interface {
	GetTaskStatus(ctx context.Context, node, upid string) (*Task, error)
}

// PollTask polls a task with exponential backoff (500ms up to 5s) until it is
// no longer running, and returns its final status.
func PollTask(ctx context.Context, client TaskStatusGetter, node, taskID string) (*Task, error) {
	backoff := 500 * time.Millisecond
	const maxBackoff = 5 * time.Second

	for {
		task, err := client.GetTaskStatus(ctx, node, taskID)
		if err != nil {
			return nil, fmt.Errorf("failed to get task status: %w", err)
		}

		if task.Status != "running" {
			return task, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// waitTimeout limits how long WaitForTaskInteractive waits; 0 means no limit
var waitTimeout time.Duration

// SetWaitTimeout sets the limit used by WaitForTaskInteractive (from --timeout)
func SetWaitTimeout(d time.Duration) {
	waitTimeout = d
}

// WaitForTaskInteractive waits like WaitForTaskSuccess, but stops waiting on
// Ctrl-C or after the --timeout limit. Stopping only stops prox from waiting;
// the task keeps running on the server, and the error says how to follow it.
func WaitForTaskInteractive(ctx context.Context, client TaskStatusGetter, node, taskID string) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	if waitTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, waitTimeout)
		defer cancel()
	}

	err := WaitForTaskSuccess(ctx, client, node, taskID)
	if err != nil && ctx.Err() != nil {
		reason := "interrupted"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			reason = fmt.Sprintf("timed out after %s", waitTimeout)
		}
		return fmt.Errorf("stopped waiting (%s); the task is still running on the server. Follow it with: prox logs --follow %s", reason, taskID)
	}
	return err
}

// WaitForTaskSuccess waits for a task to finish and returns an error if it
// did not finish with exit status OK.
func WaitForTaskSuccess(ctx context.Context, client TaskStatusGetter, node, taskID string) error {
	task, err := PollTask(ctx, client, node, taskID)
	if err != nil {
		return err
	}
	if task.ExitCode != "OK" {
		return fmt.Errorf("task failed with exit code: %s", task.ExitCode)
	}
	return nil
}
