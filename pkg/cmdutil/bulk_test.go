package cmdutil

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/spf13/cobra"
)

func TestNameOrTagArgs(t *testing.T) {
	tests := []struct {
		flags   []string
		args    []string
		wantErr bool
	}{
		{nil, []string{"web"}, false},
		{nil, nil, true},
		{nil, []string{"a", "b"}, true},
		{[]string{"--tag", "web"}, nil, false},
		{[]string{"--tag", "web"}, []string{"x"}, true},
	}
	for _, tt := range tests {
		cmd := &cobra.Command{Use: "x"}
		AddTagFlags(cmd, "VM")
		if err := cmd.ParseFlags(tt.flags); err != nil {
			t.Fatal(err)
		}
		if err := NameOrTagArgs(cmd, tt.args); (err != nil) != tt.wantErr {
			t.Errorf("flags %v args %v: err = %v, wantErr %v", tt.flags, tt.args, err, tt.wantErr)
		}
	}
}

// slowTasks is a Proxmox whose tasks take a moment, and which records how many
// were being waited for at the same time
type slowTasks struct {
	delay    time.Duration
	failing  map[string]bool // task IDs that end in an error
	mu       sync.Mutex
	inFlight int
	maxSeen  int
	polled   map[string]int
}

func (s *slowTasks) GetTaskStatus(_ context.Context, _, id string) (*c.Task, error) {
	s.mu.Lock()
	s.inFlight++
	if s.inFlight > s.maxSeen {
		s.maxSeen = s.inFlight
	}
	if s.polled == nil {
		s.polled = map[string]int{}
	}
	s.polled[id]++
	s.mu.Unlock()

	time.Sleep(s.delay)

	s.mu.Lock()
	s.inFlight--
	s.mu.Unlock()
	if s.failing[id] {
		return &c.Task{Status: "stopped", ExitCode: "ERROR: boom"}, nil
	}
	return &c.Task{Status: "stopped", ExitCode: "OK"}, nil
}

// startsTask is a target that, like vm start, starts a task and finishes it
func startsTask(tasks *slowTasks, name string, wait bool, order *[]string) BulkTarget {
	return BulkTarget{Label: name, Run: func(ctx context.Context) error {
		*order = append(*order, name) // phase one runs targets one at a time, so no lock
		return FinishTask(ctx, tasks, "pve1", "UPID:"+name, wait, name+" started")
	}}
}

func TestRunBulkContinuesAfterFailure(t *testing.T) {
	var ran []string
	target := func(name string, err error) BulkTarget {
		return BulkTarget{Label: name, Run: func(context.Context) error { ran = append(ran, name); return err }}
	}
	err := RunBulk(context.Background(), &slowTasks{}, "start", "VM", "web", []BulkTarget{
		target("a", nil), target("b", errors.New("boom")), target("c", nil),
	}, true, false)
	if err == nil || !strings.Contains(err.Error(), "1 of 3 VM(s) failed to start") {
		t.Fatalf("err = %v", err)
	}
	if strings.Join(ran, ",") != "a,b,c" {
		t.Fatalf("ran = %v", ran)
	}

	if err := RunBulk(context.Background(), &slowTasks{}, "start", "VM", "none", nil, true, false); err == nil || !strings.Contains(err.Error(), "no VMs have the tag 'none'") {
		t.Fatalf("empty: err = %v", err)
	}
}

func TestWaitingBulkStartsEveryTargetThenWaitsForAllTogether(t *testing.T) {
	tasks := &slowTasks{delay: 100 * time.Millisecond}
	var order []string
	names := []string{"a", "b", "c", "d", "e"}
	var targets []BulkTarget
	for _, n := range names {
		targets = append(targets, startsTask(tasks, n, true, &order))
	}

	start := time.Now()
	if err := RunBulk(context.Background(), tasks, "start", "VM", "web", targets, true, true); err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, "") != "abcde" {
		t.Errorf("targets started in the order %v, want a..e", order)
	}
	for _, n := range names {
		if tasks.polled["UPID:"+n] == 0 {
			t.Errorf("never waited for %s", n)
		}
	}
	if tasks.maxSeen < len(names) {
		t.Errorf("at most %d tasks were waited for at once, want all %d", tasks.maxSeen, len(names))
	}
	if elapsed := time.Since(start); elapsed > 400*time.Millisecond {
		t.Errorf("took %s: five 100ms tasks should overlap, not add up", elapsed)
	}
}

func TestWaitingBulkReportsEachFailedTaskAndWaitsForTheRest(t *testing.T) {
	tasks := &slowTasks{failing: map[string]bool{"UPID:b": true, "UPID:d": true}}
	var order []string
	var targets []BulkTarget
	for _, n := range []string{"a", "b", "c", "d"} {
		targets = append(targets, startsTask(tasks, n, true, &order))
	}
	err := RunBulk(context.Background(), tasks, "start", "VM", "web", targets, true, true)
	if err == nil || !strings.Contains(err.Error(), "2 of 4 VM(s) failed to start") {
		t.Fatalf("err = %v", err)
	}
	if len(tasks.polled) != 4 {
		t.Errorf("waited for %d tasks, want all 4 even though some failed", len(tasks.polled))
	}
}

func TestWaitingBulkDoesNotWaitForATargetThatFailedToStart(t *testing.T) {
	tasks := &slowTasks{}
	var order []string
	targets := []BulkTarget{
		startsTask(tasks, "a", true, &order),
		{Label: "b", Run: func(context.Context) error { return errors.New("locked") }},
		startsTask(tasks, "c", true, &order),
	}
	err := RunBulk(context.Background(), tasks, "start", "VM", "web", targets, true, true)
	if err == nil || !strings.Contains(err.Error(), "1 of 3 VM(s) failed to start") {
		t.Fatalf("err = %v", err)
	}
	if len(tasks.polled) != 2 || tasks.polled["UPID:a"] == 0 || tasks.polled["UPID:c"] == 0 {
		t.Errorf("polled %v, want a and c only", tasks.polled)
	}
}

func TestBulkWithoutWaitNeverPolls(t *testing.T) {
	tasks := &slowTasks{}
	var order []string
	targets := []BulkTarget{startsTask(tasks, "a", false, &order), startsTask(tasks, "b", false, &order)}
	if err := RunBulk(context.Background(), tasks, "start", "VM", "web", targets, true, false); err != nil {
		t.Fatal(err)
	}
	if len(tasks.polled) != 0 {
		t.Errorf("polled %v with --no-wait", tasks.polled)
	}
}

func TestFinishTaskStillWaitsOutsideABulkRun(t *testing.T) {
	tasks := &slowTasks{}
	if err := FinishTask(context.Background(), tasks, "pve1", "UPID:solo", true, "done"); err != nil {
		t.Fatal(err)
	}
	if tasks.polled["UPID:solo"] == 0 {
		t.Error("a single command must still wait for its task itself")
	}
}
