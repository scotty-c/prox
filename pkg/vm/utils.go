package vm

import (
	"context"
	"fmt"
	"strconv"
	"time"

	c "github.com/scotty-c/prox/pkg/client"
)

// formatSize formats bytes into human-readable size
func formatSize(sizeBytes uint64) string {
	const unit = 1024
	if sizeBytes < unit {
		return fmt.Sprintf("%d B", sizeBytes)
	}
	div, exp := uint64(unit), 0
	for n := sizeBytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(sizeBytes)/float64(div), "KMGTPE"[exp])
}

// formatUptime formats uptime seconds into human-readable format
func formatUptime(uptimeSeconds int64) string {
	if uptimeSeconds <= 0 {
		return "0s"
	}

	days := uptimeSeconds / 86400
	hours := (uptimeSeconds % 86400) / 3600
	minutes := (uptimeSeconds % 3600) / 60
	seconds := uptimeSeconds % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	} else {
		return fmt.Sprintf("%ds", seconds)
	}
}

// FindByNameOrID finds a VM by either name or ID
func FindByNameOrID(client *c.ProxmoxClient, nameOrID string) (*VM, error) {
	// Get cluster resources
	resources, err := client.GetClusterResources(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster resources: %w", err)
	}

	// Try to parse as ID first
	if vmid, err := strconv.Atoi(nameOrID); err == nil {
		// Search by ID
		for _, resource := range resources {
			if resource.Type == "qemu" && resource.VMID != nil && *resource.VMID == vmid {
				vm := VM{
					ID:     int(*resource.VMID),
					Name:   resource.Name,
					Status: resource.Status,
					Node:   resource.Node,
				}
				// Add additional resource information if available
				if resource.MaxMem != nil {
					vm.MaxMemory = uint64(*resource.MaxMem)
				}
				if resource.Mem != nil {
					vm.Memory = uint64(*resource.Mem)
				}
				if resource.MaxDisk != nil {
					vm.MaxDisk = uint64(*resource.MaxDisk)
				}
				if resource.Disk != nil {
					vm.Disk = uint64(*resource.Disk)
				}
				if resource.CPU != nil {
					vm.CPUs = int(*resource.CPU * c.CPUPercentageMultiplier)
				}
				if resource.Uptime != nil {
					vm.Uptime = formatUptime(int64(*resource.Uptime))
				}
				return &vm, nil
			}
		}
	}

	// Search by name
	for _, resource := range resources {
		if resource.Type == "qemu" && resource.Name == nameOrID {
			vm := VM{
				ID:     int(*resource.VMID),
				Name:   resource.Name,
				Status: resource.Status,
				Node:   resource.Node,
			}
			// Add additional resource information if available
			if resource.MaxMem != nil {
				vm.MaxMemory = uint64(*resource.MaxMem)
			}
			if resource.Mem != nil {
				vm.Memory = uint64(*resource.Mem)
			}
			if resource.MaxDisk != nil {
				vm.MaxDisk = uint64(*resource.MaxDisk)
			}
			if resource.Disk != nil {
				vm.Disk = uint64(*resource.Disk)
			}
			if resource.CPU != nil {
				vm.CPUs = int(*resource.CPU * 100)
			}
			if resource.Uptime != nil {
				vm.Uptime = formatUptime(int64(*resource.Uptime))
			}
			return &vm, nil
		}
	}

	return nil, fmt.Errorf("VM '%s' not found", nameOrID)
}

// waitForTask waits for a Proxmox task to complete
func waitForTask(client *c.ProxmoxClient, node, taskID string) error {
	for {
		task, err := client.GetTaskStatus(context.Background(), node, taskID)
		if err != nil {
			return fmt.Errorf("failed to get task status: %w", err)
		}

		if task.Status == "stopped" {
			if task.ExitCode == "OK" {
				return nil
			}
			return fmt.Errorf("task failed with exit code: %s", task.ExitCode)
		}

		// Wait a bit before checking again
		time.Sleep(c.TaskPollIntervalSeconds * time.Second)
	}
}
