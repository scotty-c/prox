package container

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

// ListContainers lists all LXC containers
func ListContainers(ctx context.Context, node string, runningOnly bool, jsonOutput bool, tag string) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if !jsonOutput {
		output.Infoln("Retrieving LXC containers...")
	}

	// Get cluster resources
	resources, err := client.GetClusterResources(ctx)
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

		container := guest.FromResource(resource)

		containers = append(containers, container)
	}

	// Proxmox returns resources in no particular order; list them by ID
	sort.SliceStable(containers, func(i, j int) bool { return containers[i].ID < containers[j].ID })

	// Fetch IP addresses concurrently for all running containers
	guest.LookupIPs(ctx, containers, client.GetContainerIPAlternative)

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
		// Format disk usage
		diskStr := "N/A"
		if container.MaxDisk > 0 {
			diskStr = fmt.Sprintf("%s/%s (%.1f%%)", util.FormatSize(container.Disk), util.FormatSize(container.MaxDisk),
				float64(container.Disk)/float64(container.MaxDisk)*100)
		}

		t.AppendRow(table.Row{
			container.Name,
			container.ID,
			container.Status,
			container.CPUText(),
			container.MemoryText(),
			diskStr,
			container.UptimeText(),
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
