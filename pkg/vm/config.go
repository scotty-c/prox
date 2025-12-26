package vm

import (
	"context"
	"fmt"

	c "github.com/scotty-c/prox/pkg/client"
)

// Config configures a virtual machine (placeholder for future implementation)
func (v *VirtualMachine) Config(ctx context.Context, options ...VirtualMachineOption) (*Task, error) {
	// Note: This functionality would need to be implemented in the ProxmoxClient
	// For now, return an error indicating it's not implemented
	return nil, fmt.Errorf("VM configuration update not implemented in the new client")
}

// Update updates the VM configuration
func (v *VirtualMachine) Update(ctx context.Context, config map[string]interface{}) (*Task, error) {
	taskID, err := v.Client.UpdateVM(ctx, v.Node, v.ID, config)
	if err != nil {
		return nil, err
	}

	// If no task ID is returned, it was a synchronous operation
	if taskID == "" {
		return nil, nil
	}

	task := NewTask(taskID)
	return task, nil
}

// EditVm edits a VM configuration by ID and node
func EditVm(id int, node string, name string, cores int, mem int, diskSize int, options ...VirtualMachineOption) {
	client, err := c.CreateClient()
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return
	}

	// If no node specified, auto-discovery it
	if node == "" {
		fmt.Printf("Finding node for VM %d...\n", id)
		discoveredNode, err := client.GetVMNode(context.Background(), id)
		if err != nil {
			fmt.Printf("Error: Failed to find VM %d: %v\n", id, err)
			return
		}
		node = discoveredNode
		fmt.Printf("Found VM %d on node %s\n", id, node)
	}

	// Create VM instance
	vm := &VirtualMachine{
		ID:     id,
		Node:   node,
		Client: client,
	}

	// Build configuration map
	config := make(map[string]interface{})
	hasChanges := false

	if name != "" {
		config["name"] = name
		hasChanges = true
	}
	if cores > 0 {
		config["cores"] = cores
		hasChanges = true
	}
	if mem > 0 {
		config["memory"] = mem
		hasChanges = true
	}
	if diskSize > 0 {
		// For disk resize, we handle it separately as it requires a different API endpoint
		fmt.Printf("🔧 Detecting disk type for VM %d on node %s...\n", id, node)

		// Auto-detect the primary disk type
		primaryDisk, err := vm.GetPrimaryDisk(context.Background())
		if err != nil {
			fmt.Printf("Error: Failed to detect disk type for VM %d: %v\n", id, err)
			return
		}

		fmt.Printf("Found primary disk: %s\n", primaryDisk)
		fmt.Printf("🔧 Resizing disk for VM %d on node %s...\n", id, node)
		fmt.Printf("📝 Increasing disk size by: %d GB\n", diskSize)

		// Call disk resize function with auto-detected disk type
		err = vm.ResizeDisk(context.Background(), primaryDisk, fmt.Sprintf("+%dG", diskSize))
		if err != nil {
			fmt.Printf("Error: Failed to resize VM %d disk: %v\n", id, err)
			return
		}

		fmt.Printf("VM %d disk resize command issued successfully\n", id)
		fmt.Printf("Tip: Use 'prox vm list' to check the VM status\n")

		// Mark as having changes for the check, but don't add to config
		hasChanges = true
	}

	if !hasChanges {
		fmt.Printf("WARNING: No changes specified for VM %d\n", id)
		fmt.Println("� Use flags like --name, --cpu, or --memory to specify changes")
		return
	}

	// Show what changes will be made
	fmt.Printf("🔧 Updating VM %d configuration on node %s:\n", id, node)
	if name != "" {
		fmt.Printf("   • Name: %s\n", name)
	}
	if cores > 0 {
		fmt.Printf("   • CPU cores: %d\n", cores)
	}
	if mem > 0 {
		fmt.Printf("   • Memory: %d MB\n", mem)
	}
	if diskSize > 0 {
		fmt.Printf("   • Disk size: %d GB\n", diskSize)
	}

	// Only proceed with config updates if there are actual config changes (not just disk resize)
	if len(config) == 0 && diskSize > 0 {
		// Only disk resize was performed, no other config changes needed
		return
	}

	// Apply the changes
	task, err := vm.Update(context.Background(), config)
	if err != nil {
		fmt.Printf("Error: Failed to update VM %d: %v\n", id, err)
		return
	}

	if task != nil {
		fmt.Printf("VM %d update command issued successfully\n", id)
		fmt.Printf("� Task ID: %s\n", task.ID)
		fmt.Println("Tip: Use 'prox vm list' to check the update progress")
	} else {
		fmt.Printf("VM %d configuration updated successfully\n", id)
	}
}

// ResizeDisk resizes a VM disk
func (v *VirtualMachine) ResizeDisk(ctx context.Context, disk string, size string) error {
	taskID, err := v.Client.ResizeDisk(ctx, v.Node, v.ID, disk, size)
	if err != nil {
		return err
	}

	// If a task ID is returned, we could track it, but for now just return success
	if taskID != "" {
		// Task started successfully
		return nil
	}

	return nil
}

// GetDiskInfo gets information about the VM's disks
func (v *VirtualMachine) GetDiskInfo(ctx context.Context) (map[string]string, error) {
	config, err := v.Client.GetVMConfig(ctx, v.Node, v.ID)
	if err != nil {
		return nil, err
	}

	disks := make(map[string]string)

	// Common disk types in Proxmox
	diskTypes := []string{"scsi0", "scsi1", "scsi2", "scsi3", "ide0", "ide1", "ide2", "ide3", "sata0", "sata1", "sata2", "sata3", "virtio0", "virtio1", "virtio2", "virtio3"}

	for _, diskType := range diskTypes {
		if diskConfig, exists := config[diskType]; exists {
			if diskStr, ok := diskConfig.(string); ok {
				disks[diskType] = diskStr
			}
		}
	}

	return disks, nil
}

// GetPrimaryDisk returns the primary disk type for the VM
func (v *VirtualMachine) GetPrimaryDisk(ctx context.Context) (string, error) {
	disks, err := v.GetDiskInfo(ctx)
	if err != nil {
		return "", err
	}

	if len(disks) == 0 {
		return "", fmt.Errorf("no disks found for VM %d", v.ID)
	}

	// Priority order for primary disk detection
	priorities := []string{"scsi0", "ide0", "sata0", "virtio0"}

	for _, priority := range priorities {
		if _, exists := disks[priority]; exists {
			return priority, nil
		}
	}

	// If no priority disk found, return the first one
	for diskType := range disks {
		return diskType, nil
	}

	return "", fmt.Errorf("no valid disk found for VM %d", v.ID)
}
