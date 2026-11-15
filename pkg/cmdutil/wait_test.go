package cmdutil

import (
	"context"
	"strings"
	"testing"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/spf13/cobra"
)

func TestResolveWait(t *testing.T) {
	tests := []struct {
		args        []string
		defaultWait bool
		want        bool
	}{
		{nil, true, true},
		{nil, false, false},
		{[]string{"--wait"}, false, true},
		{[]string{"-w"}, false, true},
		{[]string{"--no-wait"}, true, false},
		{[]string{"--wait=false"}, true, false},
		{[]string{"--wait", "--no-wait"}, false, true},
	}
	for _, tt := range tests {
		cmd := &cobra.Command{Use: "x", Run: func(*cobra.Command, []string) {}}
		AddWaitFlags(cmd, tt.defaultWait)
		if err := cmd.ParseFlags(tt.args); err != nil {
			t.Fatal(err)
		}
		if got := ResolveWait(cmd, tt.defaultWait); got != tt.want {
			t.Errorf("args %v default %v: got %v, want %v", tt.args, tt.defaultWait, got, tt.want)
		}
	}
}

type fakeTasks struct {
	polls  int
	status c.Task
}

func (f *fakeTasks) GetTaskStatus(context.Context, string, string) (*c.Task, error) {
	f.polls++
	t := f.status
	return &t, nil
}

func TestFinishTaskDoesNotPollWithoutWait(t *testing.T) {
	tasks := &fakeTasks{}
	if err := FinishTask(context.Background(), tasks, "pve1", "UPID:x", false, "done"); err != nil {
		t.Fatal(err)
	}
	if tasks.polls != 0 {
		t.Errorf("polled %d times without --wait", tasks.polls)
	}
}

func TestFinishTaskWaitsForTheTask(t *testing.T) {
	tasks := &fakeTasks{status: c.Task{Status: "stopped", ExitCode: "OK"}}
	if err := FinishTask(context.Background(), tasks, "pve1", "UPID:x", true, "done"); err != nil {
		t.Fatal(err)
	}
	if tasks.polls != 1 {
		t.Errorf("polled %d times, want 1", tasks.polls)
	}
}

func TestFinishTaskReportsAFailedTask(t *testing.T) {
	tasks := &fakeTasks{status: c.Task{Status: "stopped", ExitCode: "command failed"}}
	err := FinishTask(context.Background(), tasks, "pve1", "UPID:x", true, "done")
	if err == nil || !strings.Contains(err.Error(), "command failed") {
		t.Fatalf("got %v", err)
	}
}
