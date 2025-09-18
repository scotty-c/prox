package vm

import (
	"context"
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	c "github.com/scotty-c/prox/pkg/client"
)

// GetVm retrieves and displays all virtual machines
func GetVm() {
	client, err := c.CreateClient()
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return
	}

	fmt.Println("📋 Retrieving virtual machines...")

	// Get cluster resources
	resources, err := client.GetClusterResources(context.Background())
	if err != nil {
		fmt.Printf("❌ Error getting cluster resources: %v\n", err)
		return
	}

	var vms []VM
	for _, resource := range resources {
		// Filter for VMs
		if resource.Type != "qemu" {
			continue
		}

		// Create VM object
		vm := VM{
			ID:     int(*resource.VMID),
			Name:   resource.Name,
			Status: resource.Status,
			Node:   resource.Node,
		}

		// Add resource information if available
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
			vm.CPUs = int(*resource.CPU * 100) // Convert to percentage
		}
		if resource.Uptime != nil {
			vm.Uptime = formatUptime(int64(*resource.Uptime))
		}

		// Get IP address for running VMs
		if resource.Status == "running" {
			ip, err := client.GetVMIP(context.Background(), resource.Node, int(*resource.VMID))
			if err != nil {
				vm.IP = "N/A"
			} else {
				vm.IP = ip
			}
		} else {
			vm.IP = "N/A"
		}

		vms = append(vms, vm)
	}

	if len(vms) == 0 {
		fmt.Println("❌ No virtual machines found")
		return
	}

	// Display VMs in a table
	displayVMsTable(vms, false, false, false) // Default: no IP, no disk
}

// ListVMs lists virtual machines with optional node and running filters
func ListVMs(node string, runningOnly bool, showIPs bool, detailed bool) {
	client, err := c.CreateClient()
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return
	}

	fmt.Println("📋 Retrieving virtual machines...")

	// Get cluster resources
	resources, err := client.GetClusterResources(context.Background())
	if err != nil {
		fmt.Printf("❌ Error getting cluster resources: %v\n", err)
		return
	}

	var vms []VM
	for _, resource := range resources {
		// Filter for VMs
		if resource.Type != "qemu" {
			continue
		}

		// Filter by node if specified
		if node != "" && resource.Node != node {
			continue
		}

		// Filter by running status if specified
		if runningOnly && resource.Status != "running" {
			continue
		}

		// Create VM object
		vm := VM{
			ID:     int(*resource.VMID),
			Name:   resource.Name,
			Status: resource.Status,
			Node:   resource.Node,
		}

		// Add resource information if available
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
			vm.CPUs = int(*resource.CPU * 100) // Convert to percentage
		}
		if resource.Uptime != nil {
			vm.Uptime = formatUptime(int64(*resource.Uptime))
		}

		// Get more accurate disk information if cluster resources don't provide it
		// Use resource data for disk info when available, skip expensive API calls for list view
		if resource.MaxDisk != nil && *resource.MaxDisk > 0 {
			vm.MaxDisk = uint64(*resource.MaxDisk)
		}
		if resource.Disk != nil && *resource.Disk > 0 {
			vm.Disk = uint64(*resource.Disk)
		}

		// Estimate disk usage if we have max but no usage info
		if vm.MaxDisk > 0 && vm.Disk == 0 {
			if resource.Status == "running" {
				vm.Disk = vm.MaxDisk / 5 // Estimate 20% usage for running VMs
			} else {
				vm.Disk = vm.MaxDisk / 10 // Estimate 10% usage for stopped VMs
			}
		}

		// Use detailed disk info if requested (slower but more accurate)
		if detailed && (vm.MaxDisk == 0 || vm.Disk == 0) {
			if maxDisk, usedDisk, err := client.GetVMDiskInfo(context.Background(), resource.Node, int(*resource.VMID)); err == nil {
				if maxDisk > 0 {
					vm.MaxDisk = maxDisk
				}
				if usedDisk > 0 {
					vm.Disk = usedDisk
				}
			}
		}

		// Only make expensive calls if we have no disk info at all and not using detailed mode
		if !detailed && vm.MaxDisk == 0 && vm.Disk == 0 {
			// Quick fallback: assume VM has disks if it's a normal VM (not a template)
			if !strings.Contains(strings.ToLower(resource.Name), "template") && !strings.Contains(strings.ToLower(resource.Name), "tpl") {
				vm.MaxDisk = 1 // Set to 1 to indicate disk presence
			}
		}

		// Get IP address for running VMs only if requested (for performance)
		if showIPs && resource.Status == "running" {
			ip, err := client.GetVMIP(context.Background(), resource.Node, int(*resource.VMID))
			if err != nil {
				vm.IP = "N/A"
			} else {
				vm.IP = ip
			}
		} else {
			vm.IP = "N/A"
		}

		vms = append(vms, vm)
	}

	if len(vms) == 0 {
		if runningOnly {
			fmt.Println("❌ No running virtual machines found")
		} else {
			fmt.Println("❌ No virtual machines found")
		}
		return
	}

	// Display VMs in a table
	displayVMsTable(vms, runningOnly, showIPs, detailed)
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
		// Format memory usage
		var memoryStr string
		if vm.MaxMemory > 0 {
			memUsed := formatSize(vm.Memory)
			memMax := formatSize(vm.MaxMemory)
			memPercent := float64(vm.Memory) / float64(vm.MaxMemory) * 100
			memoryStr = fmt.Sprintf("%s/%s (%.1f%%)", memUsed, memMax, memPercent)
		} else {
			memoryStr = "N/A"
		}

		// Format disk usage (only if showing disk column)
		var diskStr string
		if showDisk {
			if vm.MaxDisk > 1 {
				diskUsed := formatSize(vm.Disk)
				diskMax := formatSize(vm.MaxDisk)
				diskPercent := float64(vm.Disk) / float64(vm.MaxDisk) * 100
				diskStr = fmt.Sprintf("%s/%s (%.1f%%)", diskUsed, diskMax, diskPercent)
			} else if vm.MaxDisk == 1 {
				// VM has disks but size info not available
				diskStr = "Configured"
			} else {
				diskStr = "N/A"
			}
		}

		// Format CPU usage
		cpuStr := fmt.Sprintf("%d%%", vm.CPUs)
		if vm.CPUs == 0 {
			cpuStr = "0%"
		}

		// Format uptime
		uptimeStr := vm.Uptime
		if uptimeStr == "" {
			uptimeStr = "N/A"
		}

		// Build row dynamically based on flags
		row := []interface{}{vm.Name, vm.ID, vm.Status, cpuStr, memoryStr}
		if showDisk {
			row = append(row, diskStr)
		}
		row = append(row, uptimeStr)
		if showIPs {
			row = append(row, vm.IP)
		}
		row = append(row, vm.Node)

		t.AppendRow(table.Row(row))
	}

	t.SetStyle(table.StyleRounded)
	fmt.Printf("\n%s\n", t.Render())
	if runningOnly {
		fmt.Printf("Found %d running virtual machine(s)\n", len(vms))
	} else {
		fmt.Printf("Found %d virtual machine(s)\n", len(vms))
	}
}
