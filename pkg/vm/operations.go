package vm

import (
	"context"
	"fmt"
	"time"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/lookup"
	"github.com/scotty-c/prox/pkg/output"
)

// vmLabel names a VM for messages, like "web (ID 100)". It reads the cluster
// resources the client has already fetched; with no name it is just "ID 100".
func vmLabel(ctx context.Context, client c.ProxmoxClientInterface, id int) string {
	if resources, err := client.GetClusterResources(ctx); err == nil {
		for _, r := range resources {
			if r.Type == "qemu" && r.VMID != nil && *r.VMID == id && r.Name != "" {
				return fmt.Sprintf("%s (ID %d)", r.Name, id)
			}
		}
	}
	return fmt.Sprintf("ID %d", id)
}

// Shutdown shuts down a virtual machine
func (v *VirtualMachine) Shutdown(ctx context.Context) (*Task, error) {
	upid, err := v.Client.StopVM(ctx, v.Node, v.ID)
	if err != nil {
		return nil, err
	}

	task := NewTask(upid)
	return task, nil
}

// ShutdownVMWithOptions shuts down a VM by ID and node. With force, the VM is
// stopped immediately instead of being asked to shut down cleanly.
func ShutdownVMWithOptions(ctx context.Context, id int, node string, wait bool, force bool, client c.ProxmoxClientInterface) error {
	if client == nil {
		var err error
		client, err = c.CreateClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
	}

	// If no node specified, auto-discover it
	if node == "" {
		output.Info("Finding node for VM %d...\n", id)
		discoveredNode, err := client.GetVMNode(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to find VM %d: %w", id, err)
		}
		node = discoveredNode
		output.Info("Found VM %d on node %s\n", id, node)
	}

	vm := &VirtualMachine{
		ID:     id,
		Node:   node,
		Client: client,
	}

	label := vmLabel(ctx, client, id)
	var task *Task
	var err error
	if force {
		output.Info("Force stopping VM %s on node %s...\n", label, node)
		var upid string
		upid, err = client.ForceStopVM(ctx, node, id)
		task = NewTask(upid)
	} else {
		output.Info("Shutting down VM %s on node %s...\n", label, node)
		task, err = vm.Shutdown(ctx)
	}
	if err != nil {
		return fmt.Errorf("failed to shutdown VM %d: %w", id, err)
	}

	if err := cmdutil.FinishTask(ctx, client, node, task.ID, wait, fmt.Sprintf("VM %s shut down", label)); err != nil {
		return fmt.Errorf("shutdown failed: %w", err)
	}
	return nil
}

// Start starts a virtual machine
func (v *VirtualMachine) Start(ctx context.Context) (*Task, error) {
	upid, err := v.Client.StartVM(ctx, v.Node, v.ID)
	if err != nil {
		return nil, err
	}

	task := NewTask(upid)
	return task, nil
}

// StartVMWithWait starts a VM by ID and node with optional wait
func StartVMWithWait(ctx context.Context, id int, node string, wait bool, client c.ProxmoxClientInterface) error {
	if client == nil {
		var err error
		client, err = c.CreateClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
	}

	// If no node specified, auto-discover it
	if node == "" {
		output.Info("Finding node for VM %d...\n", id)
		discoveredNode, err := client.GetVMNode(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to find VM %d: %w", id, err)
		}
		node = discoveredNode
		output.Info("Found VM %d on node %s\n", id, node)
	}

	label := vmLabel(ctx, client, id)
	output.Info("Starting VM %s on node %s...\n", label, node)

	vm := &VirtualMachine{
		ID:     id,
		Node:   node,
		Client: client,
	}

	task, err := vm.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start VM %d: %w", id, err)
	}

	if err := cmdutil.FinishTask(ctx, client, node, task.ID, wait, fmt.Sprintf("VM %s started", label)); err != nil {
		return fmt.Errorf("start failed: %w", err)
	}
	return nil
}

// Clone clones a virtual machine
func (v *VirtualMachine) Clone(ctx context.Context, name string, newId int, full bool) (*Task, error) {
	upid, err := v.Client.CloneVM(ctx, v.Node, v.ID, newId, name, full)
	if err != nil {
		return nil, err
	}

	task := NewTask(upid)
	return task, nil
}

// CloneVMWithWait clones a VM by ID and node with optional wait
func CloneVMWithWait(ctx context.Context, id int, node string, name string, newId int, full bool, wait bool) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// If no node specified, auto-discover it
	if node == "" {
		output.Info("Finding node for source VM %d...\n", id)
		discoveredNode, err := client.GetVMNode(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to find source VM %d: %w", id, err)
		}
		node = discoveredNode
		output.Info("Found source VM %d on node %s\n", id, node)
	}

	vm := &VirtualMachine{
		ID:     id,
		Node:   node,
		Client: client,
	}

	if newId == 0 {
		if newId, err = client.GetNextVMID(ctx); err != nil {
			return fmt.Errorf("failed to get the next free VM ID: %w", err)
		}
		output.Info("Using the next free VM ID: %d\n", newId)
	} else {
		output.Info("Checking if VM ID %d is available...\n", newId)
		if err := lookup.CheckIDFree(ctx, client, newId); err != nil {
			return err
		}
	}

	cloneType := "linked"
	if full {
		cloneType = "full"
	}
	output.Info("Cloning VM %d to new VM %d (%s) on node %s using %s clone...\n", id, newId, name, node, cloneType)

	startTime := time.Now()
	task, err := vm.Clone(ctx, name, newId, full)
	if err != nil {
		return fmt.Errorf("failed to clone VM %d: %w", id, err)
	}

	output.Result("VM %d clone command issued successfully\n", id)
	output.Result("Task ID: %s\n", task.ID)

	if wait {
		output.Infoln("Waiting for clone operation to complete...")

		// Wait for the clone task to complete
		err = cmdutil.WaitForTask(ctx, client, node, task.ID)
		if err != nil {
			return fmt.Errorf("clone failed: %w", err)
		}

		duration := time.Since(startTime)
		output.Result("VM clone completed successfully in %s!\n", duration.Round(time.Millisecond))
		output.Result("New VM: %s (ID: %d)\n", name, newId)
	} else {
		output.Result("VM clone started successfully\n")
		output.Result("New VM: %s (ID: %d)\n", name, newId)
	}

	output.Info("Tip: Use 'prox vm list' to check the new VM\n")
	return nil
}

// Delete deletes a virtual machine
func (v *VirtualMachine) Delete(ctx context.Context) (*Task, error) {
	upid, err := v.Client.DeleteVM(ctx, v.Node, v.ID)
	if err != nil {
		return nil, err
	}

	task := NewTask(upid)
	return task, nil
}

// DeleteVMWithWait deletes a VM by ID and node, optionally waiting for the
// deletion to finish
func DeleteVMWithWait(ctx context.Context, id int, node string, wait bool) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// If no node specified, auto-discover it
	if node == "" {
		output.Info("Finding node for VM %d...\n", id)
		discoveredNode, err := client.GetVMNode(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to find VM %d: %w", id, err)
		}
		node = discoveredNode
		output.Info("Found VM %d on node %s\n", id, node)
	}

	label := vmLabel(ctx, client, id)
	output.Info("Deleting VM %s on node %s...\n", label, node)

	vm := &VirtualMachine{
		ID:     id,
		Node:   node,
		Client: client,
	}

	task, err := vm.Delete(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete VM %d: %w", id, err)
	}

	if err := cmdutil.FinishTask(ctx, client, node, task.ID, wait, fmt.Sprintf("VM %s deleted", label)); err != nil {
		return fmt.Errorf("deletion failed: %w", err)
	}
	return nil
}

// MigrateVMWithWait migrates a VM, optionally returning as soon as the
// migration task has started
func MigrateVMWithWait(ctx context.Context, id int, sourceNode, targetNode string, online bool, withLocalDisks bool, wait bool) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Auto-discover source node if not specified
	if sourceNode == "" {
		output.Info("Discovering source node for VM %d...\n", id)
		discoveredNode, err := client.GetVMNode(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to find VM %d: %w", id, err)
		}
		sourceNode = discoveredNode
		output.Info("Found VM %d on node: %s\n", id, sourceNode)
	}

	// Prepare migration options
	options := make(map[string]interface{})

	// Set migration type (online/offline)
	if online {
		options["online"] = c.ProxmoxBoolTrue
		output.Info("Performing online migration (VM will continue running)\n")
	} else {
		output.Info("Performing offline migration (VM will be stopped)\n")
	}

	// Handle local disks
	if withLocalDisks {
		options["with-local-disks"] = c.ProxmoxBoolTrue
		output.Info("Including local disks in migration\n")
	}

	output.Info("Migrating VM %d from %s to %s...\n", id, sourceNode, targetNode)

	startTime := time.Now()
	// Start the migration
	taskID, err := client.MigrateVM(ctx, sourceNode, id, targetNode, options)
	if err != nil {
		return fmt.Errorf("failed to start migration: %w", err)
	}

	output.Result("Migration task started: %s\n", taskID)
	if !wait {
		output.Info("Tip: Follow progress with 'prox logs --follow %s'\n", taskID)
		return nil
	}
	output.Infoln("Waiting for migration to complete...")

	// Wait for the migration task to complete
	err = cmdutil.WaitForTask(ctx, client, sourceNode, taskID)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	duration := time.Since(startTime)
	output.Result("VM %d migration completed successfully in %s!\n", id, duration.Round(time.Millisecond))
	output.Result("VM %d is now on node: %s\n", id, targetNode)

	if online {
		output.Resultln("VM remained online during migration")
	} else {
		output.Info("Tip: Use 'prox vm start %d' to start the VM on the new node\n", id)
	}
	return nil
}

// StartVMByNameOrIDWithWait starts a VM by name or ID with optional wait.
// If node is not empty, only VMs on that node are considered.
func StartVMByNameOrIDWithWait(ctx context.Context, nameOrID string, node string, wait bool) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Find the VM by name or ID
	vm, err := FindByNameOrIDOnNode(ctx, client, nameOrID, node)
	if err != nil {
		return fmt.Errorf("failed to find VM: %w", err)
	}

	// Start the VM
	if err := StartVMWithWait(ctx, vm.ID, vm.Node, wait, client); err != nil {
		return fmt.Errorf("failed to start VM: %w", err)
	}
	return nil
}

// ShutdownVMByNameOrIDWithOptions shuts down a VM by name or ID. With force,
// the VM is stopped immediately instead of being asked to shut down cleanly.
func ShutdownVMByNameOrIDWithOptions(ctx context.Context, nameOrID string, node string, wait bool, force bool) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Find the VM by name or ID
	vm, err := FindByNameOrIDOnNode(ctx, client, nameOrID, node)
	if err != nil {
		return fmt.Errorf("failed to find VM: %w", err)
	}

	// Shutdown the VM
	if err := ShutdownVMWithOptions(ctx, vm.ID, vm.Node, wait, force, client); err != nil {
		return fmt.Errorf("failed to shutdown VM: %w", err)
	}
	return nil
}

// CloneVMByNameOrIDWithWait clones a VM by name or ID with optional wait.
// If node is not empty, only source VMs on that node are considered.
func CloneVMByNameOrIDWithWait(ctx context.Context, sourceNameOrID string, node string, name string, newID int, full bool, wait bool) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Find the source VM by name or ID
	vm, err := FindByNameOrIDOnNode(ctx, client, sourceNameOrID, node)
	if err != nil {
		return fmt.Errorf("failed to find source VM: %w", err)
	}

	// Clone the VM
	return CloneVMWithWait(ctx, vm.ID, vm.Node, name, newID, full, wait)
}

// MigrateVMByNameOrIDWithWait migrates a VM by name or ID, optionally without
// waiting for the migration to finish
func MigrateVMByNameOrIDWithWait(ctx context.Context, nameOrID string, sourceNode, targetNode string, online bool, withLocalDisks bool, wait bool) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Find the VM by name or ID
	vm, err := FindByNameOrIDOnNode(ctx, client, nameOrID, sourceNode)
	if err != nil {
		return fmt.Errorf("failed to find VM: %w", err)
	}

	// Use the discovered node if sourceNode is not specified
	if sourceNode == "" {
		sourceNode = vm.Node
	}

	// Migrate the VM
	return MigrateVMWithWait(ctx, vm.ID, sourceNode, targetNode, online, withLocalDisks, wait)
}

// RebootVMByNameOrID reboots a running VM cleanly (ACPI or guest agent).
// If node is not empty, only VMs on that node are considered.
func RebootVMByNameOrID(ctx context.Context, nameOrID, node string, wait bool) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	vm, err := FindByNameOrIDOnNode(ctx, client, nameOrID, node)
	if err != nil {
		return fmt.Errorf("failed to find VM: %w", err)
	}
	if vm.Status != "running" {
		return fmt.Errorf("VM %s is not running (status: %s); use 'prox vm start %s'", vm.Name, vm.Status, vm.Name)
	}

	output.Info("Rebooting VM %s (ID: %d) on node %s...\n", vm.Name, vm.ID, vm.Node)
	taskID, err := client.RebootGuest(ctx, vm.Node, c.GuestVM, vm.ID)
	if err != nil {
		return fmt.Errorf("failed to reboot VM %d: %w", vm.ID, err)
	}
	if err := cmdutil.FinishTask(ctx, client, vm.Node, taskID, wait, fmt.Sprintf("VM %s rebooted", vm.Name)); err != nil {
		return fmt.Errorf("reboot failed: %w", err)
	}
	return nil
}
