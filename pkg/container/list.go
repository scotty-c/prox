package container

import (
	"context"
	"fmt"
	"sort"

	"github.com/jedib0t/go-pretty/v6/table"
	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/scotty-c/prox/pkg/util"
)

// fetchContainerIPs looks up the IP address of every running container,
// several at a time, and stores it in the container. Containers without a
// known address show as N/A.
func fetchContainerIPs(client c.ProxmoxClientInterface, containers []Container) {
	var running []int
	for i := range containers {
		if containers[i].Status == "running" {
			running = append(running, i)
		}
	}
	util.ForEach(len(running), c.MaxConcurrentIPLookups, func(k int) {
		ct := &containers[running[k]]
		ip, err := client.GetContainerIPAlternative(context.Background(), ct.Node, ct.ID)
		if err != nil || ip == "" {
			ip = "N/A" // shown as-is in tables and JSON
		}
		ct.IP = ip
	})
}

// ListContainers lists all LXC containers
func ListContainers(node string, runningOnly bool, jsonOutput bool, tag string) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if !jsonOutput {
		output.Infoln("Retrieving LXC containers...")
	}

	// Get cluster resources
	resources, err := client.GetClusterResources(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get cluster resources: %w", err)
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

		// Skip if VMID is nil
		if resource.VMID == nil {
			continue
		}

		// Filter by tag if specified
		if tag != "" && !util.HasTag(resource.Tags, tag) {
			continue
		}

		// Create container object
		container := Container{
			ID:     int(*resource.VMID),
			Name:   resource.Name,
			Status: resource.Status,
			Node:   resource.Node,
			Tags:   resource.Tags,
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
			container.CPUs = int(*resource.CPU * c.CPUPercentageMultiplier) // Convert to percentage
		}
		if resource.Uptime != nil {
			container.Uptime = util.FormatUptime(int64(*resource.Uptime))
		}

		// Initialize IP as N/A (will be updated concurrently for running containers)
		container.IP = "N/A"

		containers = append(containers, container)
	}

	// Proxmox returns resources in no particular order; list them by ID
	sort.SliceStable(containers, func(i, j int) bool { return containers[i].ID < containers[j].ID })

	// Fetch IP addresses concurrently for all running containers
	fetchContainerIPs(client, containers)

	if jsonOutput {
		if containers == nil {
			containers = []Container{} // print [] rather than null
		}
		if err := output.OutputJSON(containers); err != nil {
			return fmt.Errorf("failed to encode output: %w", err)
		}
		return nil
	}

	if len(containers) == 0 {
		if runningOnly {
			output.Errorln("No running containers found")
		} else {
			output.Errorln("No containers found")
		}
		return nil
	}

	// Display containers in a table
	displayContainersTable(containers, runningOnly)
	return nil
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
			memUsed := util.FormatSize(container.Memory)
			memMax := util.FormatSize(container.MaxMemory)
			memPercent := float64(container.Memory) / float64(container.MaxMemory) * 100
			memoryStr = fmt.Sprintf("%s/%s (%.1f%%)", memUsed, memMax, memPercent)
		} else {
			memoryStr = "N/A"
		}

		// Format disk usage
		var diskStr string
		if container.MaxDisk > 0 {
			diskUsed := util.FormatSize(container.Disk)
			diskMax := util.FormatSize(container.MaxDisk)
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
	output.Result("%s\n", t.Render())
	if runningOnly {
		output.Result("Found %d running container(s)\n", len(containers))
	} else {
		output.Result("Found %d container(s)\n", len(containers))
	}
}
