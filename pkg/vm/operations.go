package vm

import (
	"context"
	"fmt"
	"time"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/output"
)

// GetID checks if a VM ID is available
func GetID(ctx context.Context, id int) error {
	client, err := c.CreateClient()
	if err != nil {
		output.Error("Error creating client: %v\n", err)
		return fmt.Errorf("failed to create client: %w", err)
	}

	resources, err := client.GetClusterResources(ctx)
	if err != nil {
		output.Error("Error getting cluster resources: %v\n", err)
		return fmt.Errorf("failed to get cluster resources: %w", err)
	}

	// VMs and containers share one ID space
	for _, resource := range resources {
		if (resource.Type == "qemu" || resource.Type == "lxc") && resource.VMID != nil && *resource.VMID == id {
			kind := "VM"
			if resource.Type == "lxc" {
				kind = "container"
			}
			output.Error("Error: ID %d is already in use by %s '%s' on node %s\n", id, kind, resource.Name, resource.Node)
			output.Info("Tip: Please choose a different VM ID\n")
			return fmt.Errorf("ID %d is already in use by %s '%s' on node %s", id, kind, resource.Name, resource.Node)
		}
	}

	output.Result("VM ID %d is available\n", id)
	return nil
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

// ShutdownVM shuts down a VM by ID and node
func ShutdownVM(ctx context.Context, id int, node string) error {
	return ShutdownVMWithWait(ctx, id, node, false, nil)
}

// ShutdownVMWithWait shuts down a VM by ID and node with optional wait
func ShutdownVMWithWait(ctx context.Context, id int, node string, wait bool, client c.ProxmoxClientInterface) error {
	return ShutdownVMWithOptions(ctx, id, node, wait, false, client)
}

// ShutdownVMWithOptions shuts down a VM by ID and node. With force, the VM is
// stopped immediately instead of being asked to shut down cleanly.
func ShutdownVMWithOptions(ctx context.Context, id int, node string, wait bool, force bool, client c.ProxmoxClientInterface) error {
	if client == nil {
		var err error
		client, err = c.CreateClient()
		if err != nil {
			output.Error("Error creating client: %v\n", err)
			return fmt.Errorf("failed to create client: %w", err)
		}
	}

	// If no node specified, auto-discover it
	if node == "" {
		output.Info("Finding node for VM %d...\n", id)
		discoveredNode, err := client.GetVMNode(ctx, id)
		if err != nil {
			output.Error("Error: Failed to find VM %d: %v\n", id, err)
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

	startTime := time.Now()
	var task *Task
	var err error
	if force {
		output.Info("Force stopping VM %d on node %s...\n", id, node)
		var upid string
		upid, err = client.ForceStopVM(ctx, node, id)
		task = NewTask(upid)
	} else {
		output.Info("Shutting down VM %d on node %s...\n", id, node)
		task, err = vm.Shutdown(ctx)
	}
	if err != nil {
		output.Error("Error: Failed to shutdown VM %d: %v\n", id, err)
		return fmt.Errorf("failed to shutdown VM %d: %w", id, err)
	}

	if force {
		output.Result("VM %d force stop command issued successfully\n", id)
	} else {
		output.Result("VM %d shutdown command issued successfully\n", id)
	}
	output.Result("Task ID: %s\n", task.ID)

	if wait {
		output.Infoln("Waiting for shutdown operation to complete...")
		err = waitForTask(ctx, client, node, task.ID)
		if err != nil {
			output.Error("Error: Shutdown failed: %v\n", err)
			return fmt.Errorf("shutdown failed: %w", err)
		}
		duration := time.Since(startTime)
		output.Result("VM %d shut down successfully in %s\n", id, duration.Round(time.Millisecond))
	} else {
		output.Info("Tip: Use 'prox vm list' to check the current status\n")
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

// StartVM starts a VM by ID and node
func StartVM(ctx context.Context, id int, node string) error {
	return StartVMWithWait(ctx, id, node, false, nil)
}

// StartVMWithWait starts a VM by ID and node with optional wait
func StartVMWithWait(ctx context.Context, id int, node string, wait bool, client c.ProxmoxClientInterface) error {
	if client == nil {
		var err error
		client, err = c.CreateClient()
		if err != nil {
			output.Error("Error creating client: %v\n", err)
			return fmt.Errorf("failed to create client: %w", err)
		}
	}

	// If no node specified, auto-discover it
	if node == "" {
		output.Info("Finding node for VM %d...\n", id)
		discoveredNode, err := client.GetVMNode(ctx, id)
		if err != nil {
			output.Error("Error: Failed to find VM %d: %v\n", id, err)
			return fmt.Errorf("failed to find VM %d: %w", id, err)
		}
		node = discoveredNode
		output.Info("Found VM %d on node %s\n", id, node)
	}

	output.Info("Starting VM %d on node %s...\n", id, node)

	vm := &VirtualMachine{
		ID:     id,
		Node:   node,
		Client: client,
	}

	startTime := time.Now()
	task, err := vm.Start(ctx)
	if err != nil {
		output.Error("Error: Failed to start VM %d: %v\n", id, err)
		return fmt.Errorf("failed to start VM %d: %w", id, err)
	}

	output.Result("VM %d start command issued successfully\n", id)
	output.Result("Task ID: %s\n", task.ID)

	if wait {
		output.Infoln("Waiting for start operation to complete...")
		err = waitForTask(ctx, client, node, task.ID)
		if err != nil {
			output.Error("Error: Start failed: %v\n", err)
			return fmt.Errorf("start failed: %w", err)
		}
		duration := time.Since(startTime)
		output.Result("VM %d started successfully in %s\n", id, duration.Round(time.Millisecond))
	} else {
		output.Info("Tip: Use 'prox vm list' to check the current status\n")
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

// CloneVM clones a VM by ID and node
func CloneVM(ctx context.Context, id int, node string, name string, newId int, full bool) error {
	return CloneVMWithWait(ctx, id, node, name, newId, full, true)
}

// CloneVMWithWait clones a VM by ID and node with optional wait
func CloneVMWithWait(ctx context.Context, id int, node string, name string, newId int, full bool, wait bool) error {
	client, err := c.CreateClient()
	if err != nil {
		output.Error("Error creating client: %v\n", err)
		return fmt.Errorf("failed to create client: %w", err)
	}

	// If no node specified, auto-discover it
	if node == "" {
		output.Info("Finding node for source VM %d...\n", id)
		discoveredNode, err := client.GetVMNode(ctx, id)
		if err != nil {
			output.Error("Error: Failed to find source VM %d: %v\n", id, err)
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

	output.Info("Checking if VM ID %d is available...\n", newId)
	// check to see if the new id is already in use if so the program will exit
	if err := GetID(ctx, newId); err != nil {
		return err
	}

	cloneType := "linked"
	if full {
		cloneType = "full"
	}
	output.Info("Cloning VM %d to new VM %d (%s) on node %s using %s clone...\n", id, newId, name, node, cloneType)

	startTime := time.Now()
	task, err := vm.Clone(ctx, name, newId, full)
	if err != nil {
		output.Error("Error: Failed to clone VM %d: %v\n", id, err)
		return fmt.Errorf("failed to clone VM %d: %w", id, err)
	}

	output.Result("VM %d clone command issued successfully\n", id)
	output.Result("Task ID: %s\n", task.ID)

	if wait {
		output.Infoln("Waiting for clone operation to complete...")

		// Wait for the clone task to complete
		err = waitForTask(ctx, client, node, task.ID)
		if err != nil {
			output.Error("Error: Clone failed: %v\n", err)
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

// DeleteVM deletes a VM by ID and node without waiting for it to finish
func DeleteVM(ctx context.Context, id int, node string) error {
	return DeleteVMWithWait(ctx, id, node, false)
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

	output.Info("Deleting VM %d on node %s...\n", id, node)
	output.Infoln("WARNING: This action cannot be undone!")

	vm := &VirtualMachine{
		ID:     id,
		Node:   node,
		Client: client,
	}

	task, err := vm.Delete(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete VM %d: %w", id, err)
	}

	output.Result("VM %d deletion command issued successfully\n", id)
	output.Result("Task ID: %s\n", task.ID)

	if wait {
		output.Infoln("Waiting for deletion to complete...")
		if err := waitForTask(ctx, client, node, task.ID); err != nil {
			return fmt.Errorf("deletion failed: %w", err)
		}
		output.Result("VM %d deleted\n", id)
		return nil
	}

	output.Info("Tip: Use 'prox vm list' to verify the VM has been removed\n")
	return nil
}

// MigrateVM migrates a VM from one node to another
func MigrateVM(ctx context.Context, id int, sourceNode, targetNode string, online bool, withLocalDisks bool) error {
	return MigrateVMWithWait(ctx, id, sourceNode, targetNode, online, withLocalDisks, true)
}

// MigrateVMWithWait migrates a VM, optionally returning as soon as the
// migration task has started
func MigrateVMWithWait(ctx context.Context, id int, sourceNode, targetNode string, online bool, withLocalDisks bool, wait bool) error {
	client, err := c.CreateClient()
	if err != nil {
		output.Error("Error: Error creating client: %v\n", err)
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Auto-discover source node if not specified
	if sourceNode == "" {
		output.Info("Discovering source node for VM %d...\n", id)
		discoveredNode, err := client.GetVMNode(ctx, id)
		if err != nil {
			output.Error("Error: Failed to find VM %d: %v\n", id, err)
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
		output.Error("Error: Failed to start migration: %v\n", err)
		return fmt.Errorf("failed to start migration: %w", err)
	}

	output.Result("Migration task started: %s\n", taskID)
	if !wait {
		output.Info("Tip: Follow progress with 'prox logs --follow %s'\n", taskID)
		return nil
	}
	output.Infoln("Waiting for migration to complete...")

	// Wait for the migration task to complete
	err = waitForTask(ctx, client, sourceNode, taskID)
	if err != nil {
		output.Error("Error: Migration failed: %v\n", err)
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

// StartVMByNameOrID starts a VM by name or ID
func StartVMByNameOrID(nameOrID string) error {
	return StartVMByNameOrIDWithWait(nameOrID, "", false)
}

// StartVMByNameOrIDWithWait starts a VM by name or ID with optional wait.
// If node is not empty, only VMs on that node are considered.
func StartVMByNameOrIDWithWait(nameOrID string, node string, wait bool) error {
	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		output.ClientError(err)
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

// ShutdownVMByNameOrID shuts down a VM by name or ID
func ShutdownVMByNameOrID(nameOrID string) error {
	return ShutdownVMByNameOrIDWithWait(nameOrID, "", false)
}

// ShutdownVMByNameOrIDWithWait shuts down a VM by name or ID with optional wait.
// If node is not empty, only VMs on that node are considered.
func ShutdownVMByNameOrIDWithWait(nameOrID string, node string, wait bool) error {
	return ShutdownVMByNameOrIDWithOptions(nameOrID, node, wait, false)
}

// ShutdownVMByNameOrIDWithOptions shuts down a VM by name or ID. With force,
// the VM is stopped immediately instead of being asked to shut down cleanly.
func ShutdownVMByNameOrIDWithOptions(nameOrID string, node string, wait bool, force bool) error {
	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		output.ClientError(err)
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

// DeleteVMByNameOrID deletes a VM by name or ID.
// If node is not empty, only VMs on that node are considered.
func DeleteVMByNameOrID(nameOrID string, node string) error {
	return DeleteVMByNameOrIDWithWait(nameOrID, node, false)
}

// DeleteVMByNameOrIDWithWait deletes a VM by name or ID, optionally waiting
// for the deletion to finish
func DeleteVMByNameOrIDWithWait(nameOrID string, node string, wait bool) error {
	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		output.ClientError(err)
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Find the VM by name or ID
	vm, err := FindByNameOrIDOnNode(ctx, client, nameOrID, node)
	if err != nil {
		return fmt.Errorf("failed to find VM: %w", err)
	}

	// Delete the VM
	return DeleteVMWithWait(ctx, vm.ID, vm.Node, wait)
}

// CloneVMByNameOrID clones a VM by name or ID
func CloneVMByNameOrID(sourceNameOrID string, name string, newID int, full bool) error {
	return CloneVMByNameOrIDWithWait(sourceNameOrID, "", name, newID, full, true)
}

// CloneVMByNameOrIDWithWait clones a VM by name or ID with optional wait.
// If node is not empty, only source VMs on that node are considered.
func CloneVMByNameOrIDWithWait(sourceNameOrID string, node string, name string, newID int, full bool, wait bool) error {
	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		output.ClientError(err)
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

// MigrateVMByNameOrID migrates a VM by name or ID and waits for it to finish
func MigrateVMByNameOrID(nameOrID string, sourceNode, targetNode string, online bool, withLocalDisks bool) error {
	return MigrateVMByNameOrIDWithWait(nameOrID, sourceNode, targetNode, online, withLocalDisks, true)
}

// MigrateVMByNameOrIDWithWait migrates a VM by name or ID, optionally without
// waiting for the migration to finish
func MigrateVMByNameOrIDWithWait(nameOrID string, sourceNode, targetNode string, online bool, withLocalDisks bool, wait bool) error {
	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		output.ClientError(err)
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
func RebootVMByNameOrID(nameOrID, node string, wait bool) error {
	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		output.ClientError(err)
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
	start := time.Now()
	taskID, err := client.RebootGuest(ctx, vm.Node, c.GuestVM, vm.ID)
	if err != nil {
		return fmt.Errorf("failed to reboot VM %d: %w", vm.ID, err)
	}
	output.Result("Task started: %s\n", taskID)
	if !wait {
		output.Info("Tip: Follow progress with 'prox logs --follow %s'\n", taskID)
		return nil
	}
	if err := waitForTask(ctx, client, vm.Node, taskID); err != nil {
		return fmt.Errorf("reboot failed: %w", err)
	}
	output.Result("VM %s rebooted in %s\n", vm.Name, time.Since(start).Round(time.Millisecond))
	return nil
}
