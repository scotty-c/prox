package vm

import (
	"context"
	"fmt"
	"sort"

	"github.com/jedib0t/go-pretty/v6/table"
	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/guest"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/scotty-c/prox/pkg/util"
)

// ListVMsOptions contains options for listing virtual machines
type ListVMsOptions struct {
	Node        string // Filter by specific node (empty for all nodes)
	RunningOnly bool   // Show only running VMs
	ShowIPs     bool   // Fetch and display IP addresses (slower)
	Detailed    bool   // Show detailed disk information (slower)
	JSONOutput  bool   // Output as JSON instead of table
	Tag         string // Filter by tag (empty for all VMs)
}

// fillDiskDetails fills in what the cluster listing lacks about disks, for
// several VMs at a time: the total size across all of a VM's disks (the
// listing only reports the boot disk) and, for a running VM with the guest
// agent, the space used. A VM that is not running needs only its configuration
// (two fewer requests): there is no status or guest agent to ask.
func fillDiskDetails(ctx context.Context, client c.ProxmoxClientInterface, vms []VM) {
	var todo []int
	for i, vm := range vms {
		if vm.MaxDisk == 0 || vm.Disk == 0 {
			todo = append(todo, i)
		}
	}
	util.ForEach(len(todo), c.MaxConcurrentLookups, func(k int) {
		vm := &vms[todo[k]]
		var maxDisk, usedDisk uint64
		if vm.Status == "running" {
			if total, used, err := client.GetVMDiskInfo(ctx, vm.Node, vm.ID); err == nil {
				maxDisk, usedDisk = total, used
			}
		} else if config, err := client.GetVMConfig(ctx, vm.Node, vm.ID); err == nil {
			maxDisk, usedDisk = client.GetVMDiskInfoFrom(ctx, vm.Node, vm.ID, config, nil)
		}
		if maxDisk > 0 {
			vm.MaxDisk = maxDisk
		}
		if usedDisk > 0 {
			vm.Disk = usedDisk
		}
	})
}

// ListVMs lists virtual machines with the provided options
func ListVMs(ctx context.Context, opts ListVMsOptions) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if !opts.JSONOutput {
		output.Infoln("Retrieving virtual machines...")
	}

	// Get cluster resources
	resources, err := client.GetClusterResources(ctx)
	if err != nil {
		return fmt.Errorf("failed to get cluster resources: %w", err)
	}

	var vms []VM
	for _, resource := range resources {
		// Filter for VMs
		if resource.Type != "qemu" {
			continue
		}

		// Filter by node if specified
		if opts.Node != "" && resource.Node != opts.Node {
			continue
		}

		// Filter by running status if specified
		if opts.RunningOnly && resource.Status != "running" {
			continue
		}

		// Skip if VMID is nil
		if resource.VMID == nil {
			continue
		}

		// Filter by tag if specified
		if opts.Tag != "" && !util.HasTag(resource.Tags, opts.Tag) {
			continue
		}

		vm := guest.FromResource(resource)

		vms = append(vms, vm)
	}

	if opts.Detailed {
		fillDiskDetails(ctx, client, vms)
	}

	// Proxmox returns resources in no particular order; list them by ID
	sort.SliceStable(vms, func(i, j int) bool { return vms[i].ID < vms[j].ID })

	// Fetch IPs concurrently if requested (for performance)
	if opts.ShowIPs && len(vms) > 0 {
		guest.LookupIPs(ctx, vms, client.GetVMIP)
	}

	if opts.JSONOutput {
		if vms == nil {
			vms = []VM{} // print [] rather than null
		}
		if err := output.OutputJSON(vms); err != nil {
			return fmt.Errorf("failed to encode output: %w", err)
		}
		return nil
	}

	if len(vms) == 0 {
		if opts.RunningOnly {
			output.Errorln("No running virtual machines found")
		} else {
			output.Errorln("No virtual machines found")
		}
		return nil
	}

	// Display VMs in a table
	displayVMsTable(vms, opts.RunningOnly, opts.ShowIPs, opts.Detailed)
	return nil
}

// formatDiskUsage formats disk usage for display. A used value of 0 means
// Proxmox did not report usage (it usually doesn't for VMs), so it is shown
// as N/A rather than as an empty disk.
func formatDiskUsage(used, max uint64) string {
	switch {
	case max == 0:
		return "N/A"
	case used == 0:
		return fmt.Sprintf("N/A/%s", util.FormatSize(max))
	default:
		return fmt.Sprintf("%s/%s (%.1f%%)", util.FormatSize(used), util.FormatSize(max), float64(used)/float64(max)*100)
	}
}

// displayVMsTable displays VMs in a formatted table
func displayVMsTable(vms []VM, runningOnly bool, showIPs bool, showDisk bool) {
	t := table.NewWriter()
	if runningOnly {
		t.SetTitle("Running Virtual Machines")
	} else {
		t.SetTitle("Virtual Machines")
	}

	// Build header dynamically based on flags
	header := []interface{}{"NAME", "ID", "STATUS", "CPU%", "MEMORY"}
	if showDisk {
		header = append(header, "DISK")
	}
	header = append(header, "UPTIME")
	if showIPs {
		header = append(header, "IP")
	}
	header = append(header, "NODE")
	t.AppendHeader(table.Row(header))

	for _, vm := range vms {
		// Format disk usage (only if showing disk column)
		var diskStr string
		if showDisk {
			diskStr = formatDiskUsage(vm.Disk, vm.MaxDisk)
		}

		// Build row dynamically based on flags
		row := []interface{}{vm.Name, vm.ID, vm.Status, vm.CPUText(), vm.MemoryText()}
		if showDisk {
			row = append(row, diskStr)
		}
		row = append(row, vm.UptimeText())
		if showIPs {
			row = append(row, vm.IP)
		}
		row = append(row, vm.Node)

		t.AppendRow(table.Row(row))
	}

	t.SetStyle(table.StyleRounded)
	output.Result("%s\n", t.Render())
	if runningOnly {
		output.Result("Found %d running virtual machine(s)\n", len(vms))
	} else {
		output.Result("Found %d virtual machine(s)\n", len(vms))
	}
}
