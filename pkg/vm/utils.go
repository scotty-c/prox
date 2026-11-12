package vm

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/scotty-c/prox/pkg/util"
)

// FindByNameOrID finds a VM by either name or ID
func FindByNameOrID(ctx context.Context, client c.ProxmoxClientInterface, nameOrID string) (*VM, error) {
	return FindByNameOrIDOnNode(ctx, client, nameOrID, "")
}

// FindByNameOrIDOnNode finds a VM by either name or ID. If node is not empty,
// only VMs on that node are considered.
func FindByNameOrIDOnNode(ctx context.Context, client c.ProxmoxClientInterface, nameOrID string, node string) (*VM, error) {
	// Get cluster resources
	resources, err := client.GetClusterResources(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster resources: %w", err)
	}

	onNode := func(r c.Resource) bool {
		return r.Type == "qemu" && r.VMID != nil && (node == "" || r.Node == node)
	}

	// Try to parse as ID first
	if vmid, err := strconv.Atoi(nameOrID); err == nil {
		for _, resource := range resources {
			if onNode(resource) && *resource.VMID == vmid {
				vm := vmFromResource(resource)
				return &vm, nil
			}
		}
	}

	// Search by name and collect all VM names for suggestions
	var allVMNames []string
	for _, resource := range resources {
		if !onNode(resource) {
			continue
		}
		if resource.Name == nameOrID {
			vm := vmFromResource(resource)
			return &vm, nil
		}
		// Collect VM names for fuzzy matching
		if resource.Name != "" {
			allVMNames = append(allVMNames, resource.Name)
		}
	}

	// VM not found - provide helpful suggestions
	errorMsg := fmt.Sprintf("VM '%s' not found", nameOrID)
	if node != "" {
		errorMsg = fmt.Sprintf("VM '%s' not found on node '%s'", nameOrID, node)
	}

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

	return nil, errors.New(errorMsg)
}

// vmFromResource converts a cluster resource into a VM for display and lookup
func vmFromResource(resource c.Resource) VM {
	vm := VM{
		ID:     *resource.VMID,
		Name:   resource.Name,
		Status: resource.Status,
		Node:   resource.Node,
	}
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
		vm.Uptime = util.FormatUptime(*resource.Uptime)
	}
	return vm
}

// waitForTask waits for a Proxmox task to complete, showing a spinner
func waitForTask(ctx context.Context, client c.ProxmoxClientInterface, node, taskID string) error {
	stop := output.StartSpinner(" Processing...")
	defer stop()

	return c.WaitForTaskSuccess(ctx, client, node, taskID)
}
