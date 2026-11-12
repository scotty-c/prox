package vm

import (
	"context"
	"fmt"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/output"
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

// EditVM edits a VM configuration by ID and node
func EditVM(ctx context.Context, id int, node string, name string, cores int, mem int, diskSize int, options ...VirtualMachineOption) error {
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
		output.Info("Detecting disk type for VM %d on node %s...\n", id, node)

		// Auto-detect the primary disk type
		primaryDisk, err := vm.GetPrimaryDisk(ctx)
		if err != nil {
			return fmt.Errorf("failed to detect disk type for VM %d: %w", id, err)
		}

		output.Info("Found primary disk: %s\n", primaryDisk)
		output.Info("Resizing disk for VM %d on node %s...\n", id, node)
		output.Info("Increasing disk size by: %d GB\n", diskSize)

		// Call disk resize function with auto-detected disk type
		if err := vm.ResizeDisk(ctx, primaryDisk, fmt.Sprintf("+%dG", diskSize)); err != nil {
			return fmt.Errorf("failed to resize VM %d disk: %w", id, err)
		}

		output.Result("VM %d disk resize command issued successfully\n", id)
		output.Info("Tip: Use 'prox vm list' to check the VM status\n")

		// Mark as having changes for the check, but don't add to config
		hasChanges = true
	}

	if !hasChanges {
		output.Error("WARNING: No changes specified for VM %d\n", id)
		output.Error("Tip: Use flags like --name, --cpu, --memory, or --disk to specify changes\n")
		return nil
	}

	// Show what changes will be made
	output.Info("Updating VM %d configuration on node %s:\n", id, node)
	if name != "" {
		output.Info("   • Name: %s\n", name)
	}
	if cores > 0 {
		output.Info("   • CPU cores: %d\n", cores)
	}
	if mem > 0 {
		output.Info("   • Memory: %d MB\n", mem)
	}
	if diskSize > 0 {
		output.Info("   • Disk size increase: %d GB\n", diskSize)
	}

	// Only proceed with config updates if there are actual config changes (not just disk resize)
	if len(config) == 0 {
		return nil
	}

	// Apply the changes
	task, err := vm.Update(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to update VM %d: %w", id, err)
	}

	if task != nil {
		output.Result("VM %d update command issued successfully\n", id)
		output.Result("Task ID: %s\n", task.ID)
		output.Info("Tip: Use 'prox vm list' to check the update progress\n")
	} else {
		output.Result("VM %d configuration updated successfully\n", id)
	}
	return nil
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

// EditVMByNameOrID edits a VM by name or ID.
// If node is not empty, only VMs on that node are considered.
func EditVMByNameOrID(nameOrID string, node string, name string, cores int, mem int, diskSize int) error {
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

	// Edit the VM
	return EditVM(ctx, vm.ID, vm.Node, name, cores, mem, diskSize)
}
