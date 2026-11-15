package vm

import (
	"context"
	"fmt"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/lookup"
	"github.com/scotty-c/prox/pkg/util"
)

// FindByNameOrID finds a VM by either name or ID
func FindByNameOrID(ctx context.Context, client c.ProxmoxClientInterface, nameOrID string) (*VM, error) {
	return FindByNameOrIDOnNode(ctx, client, nameOrID, "")
}

// FindByNameOrIDOnNode finds a VM by either name or ID. If node is not empty,
// only VMs on that node are considered.
func FindByNameOrIDOnNode(ctx context.Context, client c.ProxmoxClientInterface, nameOrID string, node string) (*VM, error) {
	resource, err := lookup.Find(ctx, client, lookup.VM, nameOrID, node)
	if err != nil {
		return nil, err
	}
	vm := vmFromResource(resource)
	return &vm, nil
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

// FindByTag returns the VMs that have tag, optionally only on node
func FindByTag(ctx context.Context, client c.ProxmoxClientInterface, tag, node string) ([]VM, error) {
	resources, err := client.GetClusterResources(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster resources: %w", err)
	}
	var vms []VM
	for _, r := range resources {
		if r.Type == "qemu" && r.VMID != nil && util.HasTag(r.Tags, tag) && (node == "" || r.Node == node) {
			vms = append(vms, vmFromResource(r))
		}
	}
	return vms, nil
}
