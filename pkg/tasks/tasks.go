// Package tasks lists the cluster's recent Proxmox tasks.
package tasks

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/output"
)

// Options selects which tasks to list
type Options struct {
	Limit    int    // at most this many, newest first; 0 means all
	Node     string // only tasks on this node
	Running  bool   // only tasks that have not finished
	Failed   bool   // only tasks that finished with an error
	ShowUPID bool   // add the task ID column to the table
}

// Task is one task as shown by 'prox tasks' (and printed with --output json)
type Task struct {
	UPID      string `json:"upid"`
	Node      string `json:"node"`
	Type      string `json:"type"`
	Target    string `json:"target,omitempty"` // the VM or container ID
	User      string `json:"user"`
	StartTime int64  `json:"start_time"`         // Unix seconds
	EndTime   int64  `json:"end_time,omitempty"` // Unix seconds; absent while running
	Status    string `json:"status"`             // running, ok or failed
	Message   string `json:"message,omitempty"`  // Proxmox's error text for a failed task
}

// classify turns Proxmox's status text into running, ok or failed
func classify(t c.ClusterTask) (status, message string) {
	switch {
	case t.EndTime == 0:
		return "running", ""
	case t.Status == "OK":
		return "ok", ""
	default:
		return "failed", t.Status
	}
}

// Select converts, filters and orders cluster tasks, newest first
func Select(all []c.ClusterTask, opts Options) []Task {
	var out []Task
	for _, ct := range all {
		if opts.Node != "" && ct.Node != opts.Node {
			continue
		}
		status, message := classify(ct)
		if opts.Running && status != "running" {
			continue
		}
		if opts.Failed && status != "failed" {
			continue
		}
		out = append(out, Task{
			UPID: ct.UPID, Node: ct.Node, Type: ct.Type, Target: ct.ID, User: ct.User,
			StartTime: ct.StartTime, EndTime: ct.EndTime, Status: status, Message: message,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].StartTime > out[j].StartTime })
	if opts.Limit > 0 && len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out
}

// List prints the cluster's recent tasks
func List(ctx context.Context, opts Options) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	all, err := client.GetClusterTasks(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}
	tasks := Select(all, opts)

	if output.Structured() {
		if tasks == nil {
			tasks = []Task{} // print [] rather than null
		}
		return output.OutputJSON(tasks)
	}
	if len(tasks) == 0 {
		output.Errorln("No matching tasks")
		return nil
	}

	t := table.NewWriter()
	t.SetTitle("Recent Tasks")
	header := table.Row{"STARTED", "DURATION", "TYPE", "TARGET", "USER", "NODE", "STATUS"}
	if opts.ShowUPID {
		header = append(header, "TASK ID")
	}
	t.AppendHeader(header)
	for _, task := range tasks {
		row := table.Row{
			time.Unix(task.StartTime, 0).Local().Format("2006-01-02 15:04:05"),
			duration(task),
			task.Type, task.Target, task.User, task.Node, statusText(task),
		}
		if opts.ShowUPID {
			row = append(row, task.UPID)
		}
		t.AppendRow(row)
	}
	t.SetStyle(table.StyleRounded)
	output.Result("%s\n", t.Render())
	output.Result("Showing %d task(s)\n", len(tasks))
	if !opts.ShowUPID {
		output.Info("Tip: add --upid (or use -o json) to see task IDs for 'prox logs --follow <id>'\n")
	}
	return nil
}

func duration(t Task) string {
	if t.EndTime == 0 {
		return "running"
	}
	return (time.Duration(t.EndTime-t.StartTime) * time.Second).String()
}

// statusText is the status for the table: OK, running, or the first line of
// Proxmox's error, shortened
func statusText(t Task) string {
	switch t.Status {
	case "ok":
		return "OK"
	case "running":
		return "running"
	}
	msg := strings.TrimSpace(strings.SplitN(t.Message, "\n", 2)[0])
	if len(msg) > 50 {
		msg = msg[:47] + "..."
	}
	return "failed: " + msg
}
