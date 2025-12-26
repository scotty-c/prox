package node

import (
	"context"
	"fmt"
	"strings"

	"github.com/scotty-c/prox/pkg/client"
)

// DescribeNode fetches details for a single node and prints a concise
// human-friendly description. Returns an error on failure.
func DescribeNode(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("node name required")
	}

	c, err := client.CreateClient()
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}

	ctx := context.Background()

	nodes, err := c.GetNodes(ctx)
	if err != nil {
		return fmt.Errorf("get nodes: %w", err)
	}

	var found *client.Node
	for i := range nodes {
		if nodes[i].Node == name || nodes[i].ID == name {
			found = &nodes[i]
			break
		}
	}

	if found == nil {
		return fmt.Errorf("node %q not found", name)
	}

	// Start sectioned output similar to VM describe
	fmt.Printf("\nNode Details\n")
	fmt.Printf("════════════════════════════════════════════════════════════════════════\n")

	// Basic Information
	fmt.Printf("Basic Information:\n")
	fmt.Printf("   Name: %s\n", found.Node)
	fmt.Printf("   ID: %s\n", found.ID)
	fmt.Printf("   Status: %s\n", found.Status)
	if found.Type != "" {
		fmt.Printf("   Type: %s\n", found.Type)
	}

	fmt.Printf("\n")

	// Resource Summary
	fmt.Printf("Resource Summary:\n")
	// Default placeholders
	var cpuStr string
	var memUsed, memTotal uint64
	var diskUsed, diskTotal uint64
	var uptimeStr string

	if resources, err := c.GetClusterResources(ctx); err == nil {
		for _, r := range resources {
			if r.Node == found.Node && r.Type == "node" {
				if r.CPU != nil {
					cpuStr = fmt.Sprintf("%.2f%%", *r.CPU*100)
				}
				if r.Mem != nil {
					memUsed = uint64(*r.Mem)
				}
				if r.MaxMem != nil {
					memTotal = uint64(*r.MaxMem)
				}
				if r.Disk != nil {
					diskUsed = uint64(*r.Disk)
				}
				if r.MaxDisk != nil {
					diskTotal = uint64(*r.MaxDisk)
				}
				if r.Uptime != nil {
					uptimeStr = formatUptime(int64(*r.Uptime))
				}
				break
			}
		}
	}

	if cpuStr != "" {
		fmt.Printf("   CPU: %s\n", cpuStr)
	}
	if memTotal > 0 {
		fmt.Printf("   Memory: %s / %s (%.1f%%)\n", formatSize(memUsed), formatSize(memTotal), (float64(memUsed)/float64(memTotal))*100)
	}
	if diskTotal > 0 {
		fmt.Printf("   Disk: %s / %s (%.1f%%)\n", formatSize(diskUsed), formatSize(diskTotal), (float64(diskUsed)/float64(diskTotal))*100)
	}
	if uptimeStr != "" {
		fmt.Printf("   Uptime: %s\n", uptimeStr)
	}

	fmt.Printf("\n")

	// Storage and Network sections are less applicable for nodes, but show what we can
	// Network: try to get a primary IP for the node
	fmt.Printf("🌐 Network:\n")
	if ip, err := c.GetNodeIP(ctx, found.Node); err == nil && ip != "N/A" {
		fmt.Printf("   IP: %s\n", ip)
	} else {
		fmt.Printf("   IP: N/A (use the Proxmox UI or node network APIs)\n")
	}

	fmt.Printf("\n")

	// Runtime Status
	fmt.Printf("📊 Runtime Status:\n")
	if cpuStr != "" {
		fmt.Printf("   CPU Usage: %s\n", cpuStr)
	}
	if memTotal > 0 {
		fmt.Printf("   Memory Usage: %s / %s (%.1f%%)\n", formatSize(memUsed), formatSize(memTotal), (float64(memUsed)/float64(memTotal))*100)
	}
	if diskTotal > 0 {
		fmt.Printf("   Disk Usage: %s / %s (%.1f%%)\n", formatSize(diskUsed), formatSize(diskTotal), (float64(diskUsed)/float64(diskTotal))*100)
	}
	if uptimeStr != "" {
		fmt.Printf("   Uptime: %s\n", uptimeStr)
	}

	fmt.Printf("\n")

	// End of node details
	// (removed stray CLI suggestion line per user request)
	fmt.Printf("\n")

	return nil
}
