package vm

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/briandowns/spinner"
	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/util"
)

// FindByNameOrID finds a VM by either name or ID
func FindByNameOrID(ctx context.Context, client c.ProxmoxClientInterface, nameOrID string) (*VM, error) {
	// Get cluster resources
	resources, err := client.GetClusterResources(ctx)
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
					vm.Uptime = util.FormatUptime(int64(*resource.Uptime))
				}
				return &vm, nil
			}
		}
	}

	// Search by name and collect all VM names for suggestions
	var allVMNames []string
	for _, resource := range resources {
		if resource.Type == "qemu" {
			if resource.Name == nameOrID {
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
					vm.Uptime = util.FormatUptime(int64(*resource.Uptime))
				}
				return &vm, nil
			}
			// Collect VM names for fuzzy matching
			if resource.Name != "" {
				allVMNames = append(allVMNames, resource.Name)
			}
		}
	}

	// VM not found - provide helpful suggestions
	errorMsg := fmt.Sprintf("VM '%s' not found", nameOrID)

	// Find similar names using fuzzy matching
	suggestions := util.FindSimilarStrings(nameOrID, allVMNames, 3)
	if len(suggestions) > 0 {
		errorMsg += "\n\nDid you mean one of these?"
		for _, suggestion := range suggestions {
			errorMsg += fmt.Sprintf("\n  • %s", suggestion)
		}
		errorMsg += "\n\nRun 'prox vm list' to see all available virtual machines"
	} else if len(allVMNames) > 0 {
		errorMsg += "\n\nRun 'prox vm list' to see all available virtual machines"
	}

	return nil, fmt.Errorf(errorMsg)
}

// waitForTask waits for a Proxmox task to complete
func waitForTask(ctx context.Context, client c.ProxmoxClientInterface, node, taskID string) error {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " Processing..."
	s.Start()
	defer s.Stop()

	// Exponential backoff configuration
	backoff := 500 * time.Millisecond // Start at 500ms
	maxBackoff := 5 * time.Second     // Cap at 5s

	for {
		task, err := client.GetTaskStatus(ctx, node, taskID)
		if err != nil {
			return fmt.Errorf("failed to get task status: %w", err)
		}

		if task.Status == "stopped" {
			if task.ExitCode == "OK" {
				return nil
			}
			return fmt.Errorf("task failed with exit code: %s", task.ExitCode)
		}

		// Wait with exponential backoff before checking again
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			// Double the backoff for next iteration, cap at maxBackoff
			backoff = backoff * 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}
