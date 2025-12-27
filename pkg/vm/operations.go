package vm

import (
	"context"
	"fmt"

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

	for _, resource := range resources {
		if resource.Type == "qemu" && resource.VMID != nil && *resource.VMID == id {
			output.Error("Error: VM ID %d is already in use on node %s\n", id, resource.Node)
			output.Info("Tip: Please choose a different VM ID\n")
			return fmt.Errorf("VM ID %d is already in use on node %s", id, resource.Node)
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

// ShutdownVm shuts down a VM by ID and node
func ShutdownVm(ctx context.Context, id int, node string) {
	client, err := c.CreateClient()
	if err != nil {
		output.Error("Error creating client: %v\n", err)
		return
	}

	// If no node specified, auto-discover it
	if node == "" {
		output.Info("Finding node for VM %d...\n", id)
		discoveredNode, err := client.GetVMNode(ctx, id)
		if err != nil {
			output.Error("Error: Failed to find VM %d: %v\n", id, err)
			return
		}
		node = discoveredNode
		output.Info("Found VM %d on node %s\n", id, node)
	}

	output.Info("Shutting down VM %d on node %s...\n", id, node)

	vm := &VirtualMachine{
		ID:     id,
		Node:   node,
		Client: client,
	}

	task, err := vm.Shutdown(ctx)
	if err != nil {
		output.Error("Error: Failed to shutdown VM %d: %v\n", id, err)
		return
	}

	output.Result("VM %d shutdown command issued successfully\n", id)
	output.Result("Task ID: %s\n", task.ID)
	output.Info("Tip: Use 'prox vms list' to check the current status\n")
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

// StartVm starts a VM by ID and node
func StartVm(ctx context.Context, id int, node string) {
	client, err := c.CreateClient()
	if err != nil {
		output.Error("Error creating client: %v\n", err)
		return
	}

	// If no node specified, auto-discover it
	if node == "" {
		output.Info("Finding node for VM %d...\n", id)
		discoveredNode, err := client.GetVMNode(ctx, id)
		if err != nil {
			output.Error("Error: Failed to find VM %d: %v\n", id, err)
			return
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

	task, err := vm.Start(ctx)
	if err != nil {
		output.Error("Error: Failed to start VM %d: %v\n", id, err)
		return
	}

	output.Result("VM %d start command issued successfully\n", id)
	output.Result("Task ID: %s\n", task.ID)
	output.Info("Tip: Use 'prox vms list' to check the current status\n")
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

// CloneVm clones a VM by ID and node
func CloneVm(ctx context.Context, id int, node string, name string, newId int, full bool) error {
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

	task, err := vm.Clone(ctx, name, newId, full)
	if err != nil {
		output.Error("Error: Failed to clone VM %d: %v\n", id, err)
		return fmt.Errorf("failed to clone VM %d: %w", id, err)
	}

	output.Result("VM %d clone command issued successfully\n", id)
	output.Result("Task ID: %s\n", task.ID)
	output.Infoln("Waiting for clone operation to complete...")

	// Wait for the clone task to complete
	err = waitForTask(ctx, client, node, task.ID)
	if err != nil {
		output.Error("Error: Clone failed: %v\n", err)
		return fmt.Errorf("clone failed: %w", err)
	}

	output.Result("VM clone completed successfully!\n")
	output.Result("New VM: %s (ID: %d)\n", name, newId)
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

// DeleteVm deletes a VM by ID and node
func DeleteVm(ctx context.Context, id int, node string) {
	client, err := c.CreateClient()
	if err != nil {
		output.Error("Error creating client: %v\n", err)
		return
	}

	// If no node specified, auto-discover it
	if node == "" {
		output.Info("Finding node for VM %d...\n", id)
		discoveredNode, err := client.GetVMNode(ctx, id)
		if err != nil {
			output.Error("Error: Failed to find VM %d: %v\n", id, err)
			return
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
		output.Error("Error: Failed to delete VM %d: %v\n", id, err)
		return
	}

	output.Result("VM %d deletion command issued successfully\n", id)
	output.Result("Task ID: %s\n", task.ID)
	output.Info("Tip: Use 'prox vms list' to verify the VM has been removed\n")
}

// MigrateVm migrates a VM from one node to another
func MigrateVm(ctx context.Context, id int, sourceNode, targetNode string, online bool, withLocalDisks bool) error {
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
		options["online"] = 1
		output.Info("Performing online migration (VM will continue running)\n")
	} else {
		output.Info("Performing offline migration (VM will be stopped)\n")
	}

	// Handle local disks
	if withLocalDisks {
		options["with-local-disks"] = 1
		output.Info("Including local disks in migration\n")
	}

	output.Info("Migrating VM %d from %s to %s...\n", id, sourceNode, targetNode)

	// Start the migration
	taskID, err := client.MigrateVM(ctx, sourceNode, id, targetNode, options)
	if err != nil {
		output.Error("Error: Failed to start migration: %v\n", err)
		return fmt.Errorf("failed to start migration: %w", err)
	}

	output.Result("Migration task started: %s\n", taskID)
	output.Infoln("Waiting for migration to complete...")

	// Wait for the migration task to complete
	err = waitForTask(ctx, client, sourceNode, taskID)
	if err != nil {
		output.Error("Error: Migration failed: %v\n", err)
		return fmt.Errorf("migration failed: %w", err)
	}

	output.Result("VM %d migration completed successfully!\n", id)
	output.Result("VM %d is now running on node: %s\n", id, targetNode)

	if online {
		output.Resultln("VM remained online during migration")
	} else {
		output.Info("Tip: Use 'prox vm start %d' to start the VM on the new node\n", id)
	}
	return nil
}

// StartVMByNameOrID starts a VM by name or ID
func StartVMByNameOrID(nameOrID string) error {
	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		output.ClientError(err)
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Find the VM by name or ID
	vm, err := FindByNameOrID(ctx, client, nameOrID)
	if err != nil {
		return fmt.Errorf("failed to find VM: %w", err)
	}

	// Start the VM
	StartVm(ctx, vm.ID, vm.Node)
	return nil
}

// ShutdownVMByNameOrID shuts down a VM by name or ID
func ShutdownVMByNameOrID(nameOrID string) error {
	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		output.ClientError(err)
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Find the VM by name or ID
	vm, err := FindByNameOrID(ctx, client, nameOrID)
	if err != nil {
		return fmt.Errorf("failed to find VM: %w", err)
	}

	// Shutdown the VM
	ShutdownVm(ctx, vm.ID, vm.Node)
	return nil
}

// DeleteVMByNameOrID deletes a VM by name or ID
func DeleteVMByNameOrID(nameOrID string) error {
	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		output.ClientError(err)
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Find the VM by name or ID
	vm, err := FindByNameOrID(ctx, client, nameOrID)
	if err != nil {
		return fmt.Errorf("failed to find VM: %w", err)
	}

	// Delete the VM
	DeleteVm(ctx, vm.ID, vm.Node)
	return nil
}

// CloneVMByNameOrID clones a VM by name or ID
func CloneVMByNameOrID(sourceNameOrID string, name string, newID int, full bool) error {
	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		output.ClientError(err)
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Find the source VM by name or ID
	vm, err := FindByNameOrID(ctx, client, sourceNameOrID)
	if err != nil {
		return fmt.Errorf("failed to find source VM: %w", err)
	}

	// Clone the VM
	return CloneVm(ctx, vm.ID, vm.Node, name, newID, full)
}

// MigrateVMByNameOrID migrates a VM by name or ID
func MigrateVMByNameOrID(nameOrID string, sourceNode, targetNode string, online bool, withLocalDisks bool) error {
	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		output.ClientError(err)
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Find the VM by name or ID
	vm, err := FindByNameOrID(ctx, client, nameOrID)
	if err != nil {
		return fmt.Errorf("failed to find VM: %w", err)
	}

	// Use the discovered node if sourceNode is not specified
	if sourceNode == "" {
		sourceNode = vm.Node
	}

	// Migrate the VM
	return MigrateVm(ctx, vm.ID, sourceNode, targetNode, online, withLocalDisks)
}
