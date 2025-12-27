package node

import (
	"context"
	"fmt"
	"strings"

	"github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/output"
)

// NodeDetails holds all information about a node for JSON output
type NodeDetails struct {
	Node     *client.Node `json:"node"`
	CPU      string       `json:"cpu,omitempty"`
	MemUsed  uint64       `json:"mem_used,omitempty"`
	MemTotal uint64       `json:"mem_total,omitempty"`
	DiskUsed uint64       `json:"disk_used,omitempty"`
	DiskTotal uint64      `json:"disk_total,omitempty"`
	Uptime   string       `json:"uptime,omitempty"`
	IP       string       `json:"ip,omitempty"`
}

// GetNodeDetails fetches detailed node information
func GetNodeDetails(name string) (*NodeDetails, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("node name required")
	}

	c, err := client.CreateClient()
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	ctx := context.Background()

	nodes, err := c.GetNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("get nodes: %w", err)
	}

	var found *client.Node
	for i := range nodes {
		if nodes[i].Node == name || nodes[i].ID == name {
			found = &nodes[i]
			break
		}
	}

	if found == nil {
		return nil, fmt.Errorf("node %q not found", name)
	}

	details := &NodeDetails{
		Node: found,
	}

	if resources, err := c.GetClusterResources(ctx); err == nil {
		for _, r := range resources {
			if r.Node == found.Node && r.Type == "node" {
				if r.CPU != nil {
					details.CPU = fmt.Sprintf("%.2f%%", *r.CPU*100)
				}
				if r.Mem != nil {
					details.MemUsed = uint64(*r.Mem)
				}
				if r.MaxMem != nil {
					details.MemTotal = uint64(*r.MaxMem)
				}
				if r.Disk != nil {
					details.DiskUsed = uint64(*r.Disk)
				}
				if r.MaxDisk != nil {
					details.DiskTotal = uint64(*r.MaxDisk)
				}
				if r.Uptime != nil {
					details.Uptime = formatUptime(int64(*r.Uptime))
				}
				break
			}
		}
	}

	if ip, err := c.GetNodeIP(ctx, found.Node); err == nil && ip != "N/A" {
		details.IP = ip
	}

	return details, nil
}

// DescribeNode fetches details for a single node and prints a concise
// human-friendly description. Returns an error on failure.
func DescribeNode(name string) error {
	details, err := GetNodeDetails(name)
	if err != nil {
		return err
	}

	// Start sectioned output similar to VM describe
	output.Result("\nNode Details\n")
	output.Result("════════════════════════════════════════════════════════════════════════\n")

	// Basic Information
	output.Result("Basic Information:\n")
	output.Result("   Name: %s\n", details.Node.Node)
	output.Result("   ID: %s\n", details.Node.ID)
	output.Result("   Status: %s\n", details.Node.Status)
	if details.Node.Type != "" {
		output.Result("   Type: %s\n", details.Node.Type)
	}

	output.Result("\n")

	// Resource Summary
	output.Result("Resource Summary:\n")

	if details.CPU != "" {
		output.Result("   CPU: %s\n", details.CPU)
	}
	if details.MemTotal > 0 {
		output.Result("   Memory: %s / %s (%.1f%%)\n", formatSize(details.MemUsed), formatSize(details.MemTotal), (float64(details.MemUsed)/float64(details.MemTotal))*100)
	}
	if details.DiskTotal > 0 {
		output.Result("   Disk: %s / %s (%.1f%%)\n", formatSize(details.DiskUsed), formatSize(details.DiskTotal), (float64(details.DiskUsed)/float64(details.DiskTotal))*100)
	}
	if details.Uptime != "" {
		output.Result("   Uptime: %s\n", details.Uptime)
	}

	output.Result("\n")

	// Storage and Network sections are less applicable for nodes, but show what we can
	// Network: try to get a primary IP for the node
	output.Result("🌐 Network:\n")
	if details.IP != "" {
		output.Result("   IP: %s\n", details.IP)
	} else {
		output.Result("   IP: N/A (use the Proxmox UI or node network APIs)\n")
	}

	output.Result("\n")

	// Runtime Status
	output.Result("📊 Runtime Status:\n")
	if details.CPU != "" {
		output.Result("   CPU Usage: %s\n", details.CPU)
	}
	if details.MemTotal > 0 {
		output.Result("   Memory Usage: %s / %s (%.1f%%)\n", formatSize(details.MemUsed), formatSize(details.MemTotal), (float64(details.MemUsed)/float64(details.MemTotal))*100)
	}
	if details.DiskTotal > 0 {
		output.Result("   Disk Usage: %s / %s (%.1f%%)\n", formatSize(details.DiskUsed), formatSize(details.DiskTotal), (float64(details.DiskUsed)/float64(details.DiskTotal))*100)
	}
	if details.Uptime != "" {
		output.Result("   Uptime: %s\n", details.Uptime)
	}

	output.Result("\n")

	// End of node details
	// (removed stray CLI suggestion line per user request)
	output.Result("\n")

	return nil
}
