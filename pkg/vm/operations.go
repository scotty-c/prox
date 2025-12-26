package vm

import (
	"context"
	"fmt"

	c "github.com/scotty-c/prox/pkg/client"
)

// GetID checks if a VM ID is available
func GetID(id int) error {
	client, err := c.CreateClient()
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return fmt.Errorf("failed to create client: %w", err)
	}

	resources, err := client.GetClusterResources(context.Background())
	if err != nil {
		fmt.Printf("Error getting cluster resources: %v\n", err)
		return fmt.Errorf("failed to get cluster resources: %w", err)
	}

	for _, resource := range resources {
		if resource.Type == "qemu" && resource.VMID != nil && *resource.VMID == id {
			fmt.Printf("❌ VM ID %d is already in use on node %s\n", id, resource.Node)
			fmt.Printf("💡 Please choose a different VM ID\n")
			return fmt.Errorf("VM ID %d is already in use on node %s", id, resource.Node)
		}
	}

	fmt.Printf("✅ VM ID %d is available\n", id)
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
func ShutdownVm(id int, node string) {
	client, err := c.CreateClient()
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return
	}

	// If no node specified, auto-discover it
	if node == "" {
		fmt.Printf("🔍 Finding node for VM %d...\n", id)
		discoveredNode, err := client.GetVMNode(context.Background(), id)
		if err != nil {
			fmt.Printf("❌ Failed to find VM %d: %v\n", id, err)
			return
		}
		node = discoveredNode
		fmt.Printf("📍 Found VM %d on node %s\n", id, node)
	}

	fmt.Printf("🛑 Shutting down VM %d on node %s...\n", id, node)

	vm := &VirtualMachine{
		ID:     id,
		Node:   node,
		Client: client,
	}

	task, err := vm.Shutdown(context.Background())
	if err != nil {
		fmt.Printf("❌ Failed to shutdown VM %d: %v\n", id, err)
		return
	}

	fmt.Printf("✅ VM %d shutdown command issued successfully\n", id)
	fmt.Printf("📋 Task ID: %s\n", task.ID)
	fmt.Println("💡 Use 'prox vms list' to check the current status")
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
func StartVm(id int, node string) {
	client, err := c.CreateClient()
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return
	}

	// If no node specified, auto-discover it
	if node == "" {
		fmt.Printf("🔍 Finding node for VM %d...\n", id)
		discoveredNode, err := client.GetVMNode(context.Background(), id)
		if err != nil {
			fmt.Printf("❌ Failed to find VM %d: %v\n", id, err)
			return
		}
		node = discoveredNode
		fmt.Printf("📍 Found VM %d on node %s\n", id, node)
	}

	fmt.Printf("🚀 Starting VM %d on node %s...\n", id, node)

	vm := &VirtualMachine{
		ID:     id,
		Node:   node,
		Client: client,
	}

	task, err := vm.Start(context.Background())
	if err != nil {
		fmt.Printf("❌ Failed to start VM %d: %v\n", id, err)
		return
	}

	fmt.Printf("✅ VM %d start command issued successfully\n", id)
	fmt.Printf("📋 Task ID: %s\n", task.ID)
	fmt.Println("💡 Use 'prox vms list' to check the current status")
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
func CloneVm(id int, node string, name string, newId int, full bool) error {
	client, err := c.CreateClient()
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return fmt.Errorf("failed to create client: %w", err)
	}

	// If no node specified, auto-discover it
	if node == "" {
		fmt.Printf("🔍 Finding node for source VM %d...\n", id)
		discoveredNode, err := client.GetVMNode(context.Background(), id)
		if err != nil {
			fmt.Printf("❌ Failed to find source VM %d: %v\n", id, err)
			return fmt.Errorf("failed to find source VM %d: %w", id, err)
		}
		node = discoveredNode
		fmt.Printf("📍 Found source VM %d on node %s\n", id, node)
	}

	vm := &VirtualMachine{
		ID:     id,
		Node:   node,
		Client: client,
	}

	fmt.Printf("🔄 Checking if VM ID %d is available...\n", newId)
	// check to see if the new id is already in use if so the program will exit
	if err := GetID(newId); err != nil {
		return err
	}

	cloneType := "linked"
	if full {
		cloneType = "full"
	}
	fmt.Printf("📋 Cloning VM %d to new VM %d (%s) on node %s using %s clone...\n", id, newId, name, node, cloneType)

	task, err := vm.Clone(context.Background(), name, newId, full)
	if err != nil {
		fmt.Printf("❌ Failed to clone VM %d: %v\n", id, err)
		return fmt.Errorf("failed to clone VM %d: %w", id, err)
	}

	fmt.Printf("✅ VM %d clone command issued successfully\n", id)
	fmt.Printf("🆕 New VM: %s (ID: %d)\n", name, newId)
	fmt.Printf("📋 Task ID: %s\n", task.ID)
	fmt.Println("💡 Use 'prox vm list' to check the cloning progress")
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
func DeleteVm(id int, node string) {
	client, err := c.CreateClient()
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return
	}

	// If no node specified, auto-discover it
	if node == "" {
		fmt.Printf("🔍 Finding node for VM %d...\n", id)
		discoveredNode, err := client.GetVMNode(context.Background(), id)
		if err != nil {
			fmt.Printf("❌ Failed to find VM %d: %v\n", id, err)
			return
		}
		node = discoveredNode
		fmt.Printf("📍 Found VM %d on node %s\n", id, node)
	}

	fmt.Printf("🗑️  Deleting VM %d on node %s...\n", id, node)
	fmt.Println("⚠️  This action cannot be undone!")

	vm := &VirtualMachine{
		ID:     id,
		Node:   node,
		Client: client,
	}

	task, err := vm.Delete(context.Background())
	if err != nil {
		fmt.Printf("❌ Failed to delete VM %d: %v\n", id, err)
		return
	}

	fmt.Printf("✅ VM %d deletion command issued successfully\n", id)
	fmt.Printf("📋 Task ID: %s\n", task.ID)
	fmt.Println("💡 Use 'prox vms list' to verify the VM has been removed")
}

// MigrateVm migrates a VM from one node to another
func MigrateVm(id int, sourceNode, targetNode string, online bool, withLocalDisks bool) error {
	client, err := c.CreateClient()
	if err != nil {
		fmt.Printf("❌ Error creating client: %v\n", err)
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Auto-discover source node if not specified
	if sourceNode == "" {
		fmt.Printf("🔍 Discovering source node for VM %d...\n", id)
		discoveredNode, err := client.GetVMNode(context.Background(), id)
		if err != nil {
			fmt.Printf("❌ Failed to find VM %d: %v\n", id, err)
			return fmt.Errorf("failed to find VM %d: %w", id, err)
		}
		sourceNode = discoveredNode
		fmt.Printf("📍 Found VM %d on node: %s\n", id, sourceNode)
	}

	// Prepare migration options
	options := make(map[string]interface{})

	// Set migration type (online/offline)
	if online {
		options["online"] = 1
		fmt.Printf("🔄 Performing online migration (VM will continue running)\n")
	} else {
		fmt.Printf("⏹️  Performing offline migration (VM will be stopped)\n")
	}

	// Handle local disks
	if withLocalDisks {
		options["with-local-disks"] = 1
		fmt.Printf("💾 Including local disks in migration\n")
	}

	fmt.Printf("🚀 Migrating VM %d from %s to %s...\n", id, sourceNode, targetNode)

	// Start the migration
	taskID, err := client.MigrateVM(context.Background(), sourceNode, id, targetNode, options)
	if err != nil {
		fmt.Printf("❌ Failed to start migration: %v\n", err)
		return fmt.Errorf("failed to start migration: %w", err)
	}

	fmt.Printf("⏳ Migration task started: %s\n", taskID)
	fmt.Println("🔄 Waiting for migration to complete...")

	// Wait for the migration task to complete
	err = waitForTask(client, sourceNode, taskID)
	if err != nil {
		fmt.Printf("❌ Migration failed: %v\n", err)
		return fmt.Errorf("migration failed: %w", err)
	}

	fmt.Printf("✅ VM %d migration completed successfully!\n", id)
	fmt.Printf("📍 VM %d is now running on node: %s\n", id, targetNode)

	if online {
		fmt.Println("🟢 VM remained online during migration")
	} else {
		fmt.Printf("💡 Use 'prox vm start %d' to start the VM on the new node\n", id)
	}
	return nil
}
