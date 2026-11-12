package client

import (
	"context"
	"testing"
	"time"
)

type fakeTasks struct {
	statuses []Task
	calls    int
}

func (f *fakeTasks) GetTaskStatus(ctx context.Context, node, upid string) (*Task, error) {
	t := f.statuses[f.calls]
	if f.calls < len(f.statuses)-1 {
		f.calls++
	}
	return &t, nil
}

func TestWaitForTaskSuccess(t *testing.T) {
	ok := &fakeTasks{statuses: []Task{{Status: "running"}, {Status: "stopped", ExitCode: "OK"}}}
	if err := WaitForTaskSuccess(context.Background(), ok, "pve", "UPID"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if ok.calls != 1 {
		t.Fatalf("expected to poll twice, polled %d extra times", ok.calls)
	}

	failed := &fakeTasks{statuses: []Task{{Status: "stopped", ExitCode: "command failed"}}}
	if err := WaitForTaskSuccess(context.Background(), failed, "pve", "UPID"); err == nil {
		t.Fatal("expected error for failed task")
	}
}

func TestPollTaskRespectsContext(t *testing.T) {
	running := &fakeTasks{statuses: []Task{{Status: "running"}}}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := PollTask(ctx, running, "pve", "UPID"); err == nil {
		t.Fatal("expected context error")
	}
}
