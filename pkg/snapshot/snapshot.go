// Package snapshot implements the snapshot commands for VMs and containers.
package snapshot

import (
	"context"
	"fmt"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/container"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/scotty-c/prox/pkg/vm"
)

// target is the VM or container whose snapshots are being managed
type target struct {
	guestType string // c.GuestVM or c.GuestContainer
	label     string // "VM" or "container", for messages
	id        int
	name      string
	node      string
}

// resolve finds the VM or container. node limits the search for VMs.
func resolve(ctx context.Context, cl c.ProxmoxClientInterface, guestType, nameOrID, node string) (*target, error) {
	switch guestType {
	case c.GuestVM:
		v, err := vm.FindByNameOrIDOnNode(ctx, cl, nameOrID, node)
		if err != nil {
			return nil, err
		}
		return &target{guestType: guestType, label: "VM", id: v.ID, name: v.Name, node: v.Node}, nil
	case c.GuestContainer:
		ct, err := container.FindByNameOrID(ctx, cl, nameOrID)
		if err != nil {
			return nil, err
		}
		return &target{guestType: guestType, label: "container", id: ct.ID, name: ct.Name, node: ct.Node}, nil
	}
	return nil, fmt.Errorf("unknown guest type %q", guestType)
}

// prepare creates a client and resolves the target
func prepare(guestType, nameOrID, node string) (context.Context, c.ProxmoxClientInterface, *target, error) {
	ctx := context.Background()
	cl, err := c.CreateClient()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create client: %w", err)
	}
	t, err := resolve(ctx, cl, guestType, nameOrID, node)
	if err != nil {
		return nil, nil, nil, err
	}
	return ctx, cl, t, nil
}

// finish prints the task ID and waits for it if asked
func finish(ctx context.Context, cl c.ProxmoxClientInterface, t *target, taskID string, wait bool, done string) error {
	output.Result("Task started: %s\n", taskID)
	if !wait {
		output.Info("Tip: Follow progress with 'prox logs --follow %s'\n", taskID)
		return nil
	}
	stop := output.StartSpinner(" Processing...")
	err := c.WaitForTaskInteractive(ctx, cl, t.node, taskID)
	stop()
	if err != nil {
		return err
	}
	output.Result("%s\n", done)
	return nil
}

// List prints the snapshots of a VM or container
func List(guestType, nameOrID, node string) error {
	ctx, cl, t, err := prepare(guestType, nameOrID, node)
	if err != nil {
		return err
	}

	snapshots, err := cl.ListSnapshots(ctx, t.node, t.guestType, t.id)
	if err != nil {
		return fmt.Errorf("failed to list snapshots: %w", err)
	}

	if output.Structured() {
		return output.OutputJSON(snapshots)
	}

	if len(snapshots) == 0 {
		output.Resultln(fmt.Sprintf("No snapshots for %s %s (ID: %d)", t.label, t.name, t.id))
		return nil
	}

	tw := table.NewWriter()
	tw.SetTitle(fmt.Sprintf("Snapshots of %s %s (ID: %d)", t.label, t.name, t.id))
	header := table.Row{"NAME", "CREATED", "PARENT", "DESCRIPTION"}
	if t.guestType == c.GuestVM {
		header = append(header, "RAM")
	}
	tw.AppendHeader(header)
	for _, s := range snapshots {
		row := table.Row{s.Name, formatSnapTime(s.SnapTime), s.Parent, s.Description}
		if t.guestType == c.GuestVM {
			ram := "no"
			if s.VMState {
				ram = "yes"
			}
			row = append(row, ram)
		}
		tw.AppendRow(row)
	}
	tw.SetStyle(table.StyleRounded)
	output.Result("\n%s\n", tw.Render())
	output.Result("Found %d snapshot(s)\n", len(snapshots))
	return nil
}

// formatSnapTime formats a Unix timestamp for the snapshot table
func formatSnapTime(ts int64) string {
	if ts <= 0 {
		return "-"
	}
	return time.Unix(ts, 0).Local().Format("2006-01-02 15:04")
}

// Create takes a snapshot. vmState also saves a VM's memory.
func Create(guestType, nameOrID, node, name, description string, vmState, wait bool) error {
	if err := c.ValidateSnapshotName(name); err != nil {
		return err
	}
	ctx, cl, t, err := prepare(guestType, nameOrID, node)
	if err != nil {
		return err
	}
	output.Info("Creating snapshot '%s' of %s %s (ID: %d)...\n", name, t.label, t.name, t.id)
	taskID, err := cl.CreateSnapshot(ctx, t.node, t.guestType, t.id, name, description, vmState)
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}
	return finish(ctx, cl, t, taskID, wait, fmt.Sprintf("Snapshot '%s' created", name))
}

// Rollback returns a VM or container to a snapshot, discarding changes since
func Rollback(guestType, nameOrID, node, name string, wait bool) error {
	ctx, cl, t, err := prepare(guestType, nameOrID, node)
	if err != nil {
		return err
	}
	output.Info("Rolling back %s %s (ID: %d) to snapshot '%s'...\n", t.label, t.name, t.id, name)
	taskID, err := cl.RollbackSnapshot(ctx, t.node, t.guestType, t.id, name)
	if err != nil {
		return fmt.Errorf("failed to roll back: %w", err)
	}
	return finish(ctx, cl, t, taskID, wait, fmt.Sprintf("Rolled back to snapshot '%s'", name))
}

// Delete removes a snapshot
func Delete(guestType, nameOrID, node, name string, wait bool) error {
	ctx, cl, t, err := prepare(guestType, nameOrID, node)
	if err != nil {
		return err
	}
	output.Info("Deleting snapshot '%s' of %s %s (ID: %d)...\n", name, t.label, t.name, t.id)
	taskID, err := cl.DeleteSnapshot(ctx, t.node, t.guestType, t.id, name)
	if err != nil {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}
	return finish(ctx, cl, t, taskID, wait, fmt.Sprintf("Snapshot '%s' deleted", name))
}
