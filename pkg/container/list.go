package container

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	c "github.com/scotty-c/prox/pkg/client"
)

// ListContainers lists all LXC containers
func ListContainers(node string, runningOnly bool, jsonOutput bool) {
	client, err := c.CreateClient()
	if err != nil {
		if jsonOutput {
			fmt.Fprintf(os.Stderr, "Error creating client: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Error creating client: %v\n", err)
		return
	}

	if !jsonOutput {
		fmt.Println("📋 Retrieving LXC containers...")
	}

	// Get cluster resources
	resources, err := client.GetClusterResources(context.Background())
	if err != nil {
		if jsonOutput {
			fmt.Fprintf(os.Stderr, "Error getting cluster resources: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("❌ Error getting cluster resources: %v\n", err)
		return
	}

	var containers []Container
	for _, resource := range resources {
		// Filter for LXC containers
		if resource.Type != "lxc" {
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

		// Create container object
		container := Container{
			ID:     int(*resource.VMID),
			Name:   resource.Name,
			Status: resource.Status,
			Node:   resource.Node,
		}

		// Add resource information if available
		if resource.MaxMem != nil {
			container.MaxMemory = uint64(*resource.MaxMem)
		}
		if resource.Mem != nil {
			container.Memory = uint64(*resource.Mem)
		}
		if resource.MaxDisk != nil {
			container.MaxDisk = uint64(*resource.MaxDisk)
		}
		if resource.Disk != nil {
			container.Disk = uint64(*resource.Disk)
		}
		if resource.CPU != nil {
			container.CPUs = int(*resource.CPU * 100) // Convert to percentage
		}
		if resource.Uptime != nil {
			container.Uptime = formatUptime(int64(*resource.Uptime))
		}

		// Get IP address for running containers
		if resource.Status == "running" {
			ip, err := client.GetContainerIPAlternative(context.Background(), resource.Node, int(*resource.VMID))
			if err != nil {
				container.IP = "N/A"
			} else {
				container.IP = ip
			}
		} else {
			container.IP = "N/A"
		}

		containers = append(containers, container)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(containers); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(containers) == 0 {
		if runningOnly {
			fmt.Println("❌ No running containers found")
		} else {
			fmt.Println("❌ No containers found")
		}
		return
	}

	// Display containers in a table
	displayContainersTable(containers, runningOnly)
}

// displayContainersTable displays containers in a formatted table
func displayContainersTable(containers []Container, runningOnly bool) {
	t := table.NewWriter()
	if runningOnly {
		t.SetTitle("Running LXC Containers")
	} else {
		t.SetTitle("LXC Containers")
	}
	t.AppendHeader(table.Row{"NAME", "ID", "STATUS", "CPU%", "MEMORY", "DISK", "UPTIME", "IP", "NODE"})

	for _, container := range containers {
		// Format memory usage
		var memoryStr string
		if container.MaxMemory > 0 {
			memUsed := formatSize(container.Memory)
			memMax := formatSize(container.MaxMemory)
			memPercent := float64(container.Memory) / float64(container.MaxMemory) * 100
			memoryStr = fmt.Sprintf("%s/%s (%.1f%%)", memUsed, memMax, memPercent)
		} else {
			memoryStr = "N/A"
		}

		// Format disk usage
		var diskStr string
		if container.MaxDisk > 0 {
			diskUsed := formatSize(container.Disk)
			diskMax := formatSize(container.MaxDisk)
			diskPercent := float64(container.Disk) / float64(container.MaxDisk) * 100
			diskStr = fmt.Sprintf("%s/%s (%.1f%%)", diskUsed, diskMax, diskPercent)
		} else {
			diskStr = "N/A"
		}

		// Format CPU usage
		cpuStr := fmt.Sprintf("%d%%", container.CPUs)
		if container.CPUs == 0 {
			cpuStr = "0%"
		}

		// Format uptime
		uptimeStr := container.Uptime
		if uptimeStr == "" {
			uptimeStr = "N/A"
		}

		t.AppendRow(table.Row{
			container.Name,
			container.ID,
			container.Status,
			cpuStr,
			memoryStr,
			diskStr,
			uptimeStr,
			container.IP,
			container.Node,
		})
	}

	t.SetStyle(table.StyleRounded)
	fmt.Printf("\n%s\n", t.Render())
	if runningOnly {
		fmt.Printf("Found %d running container(s)\n", len(containers))
	} else {
		fmt.Printf("Found %d container(s)\n", len(containers))
	}
}
