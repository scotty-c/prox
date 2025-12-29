package vm

import (
	"context"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/output"
)

// Status gets the status/IP of a virtual machine
func (v *VirtualMachine) Status(ctx context.Context, id int, targetNode string) (string, error) {
	return v.Client.GetVMIP(ctx, targetNode, id)
}

// GetIp gets the IP address of a VM by ID and node
func GetIp(ctx context.Context, id int, node string) string {
	client, err := c.CreateClient()
	if err != nil {
		return "Error getting IP"
	}

	ip, err := client.GetVMIP(ctx, node, id)
	if err != nil {
		return "Error getting IP"
	}

	return ip
}

// TestConnection tests the basic connection to Proxmox
func TestConnection() {
	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		output.Error("Error creating client: %v\n", err)
		return
	}

	output.Infoln("Testing connection to Proxmox server...")

	// Try nodes first (more compatible with older versions)
	output.Infoln("Testing nodes endpoint...")
	nodes, err := client.GetNodes(ctx)
	if err != nil {
		output.Error("Error: Error getting nodes: %v\n", err)

		// Try version endpoint as fallback
		output.Infoln("Trying version endpoint...")
		version, err2 := client.GetVersion(ctx)
		if err2 != nil {
			output.Error("Error: Error getting version: %v\n", err2)
			output.Errorln("\nBoth nodes and version endpoints failed.")
			output.Errorln("This suggests an authentication issue or API incompatibility.")
			output.Errorln("\nTroubleshooting tips:")
			output.Errorln("1. Verify credentials are correct")
			output.Errorln("2. Check if API access is enabled for the user")
			output.Errorln("3. This might be an older Proxmox version with limited API support")
			return
		}
		output.Info("✓ Connected to Proxmox version: %s\n", version.Version)
		output.Infoln("WARNING: Nodes endpoint not available, but version endpoint works")
		return
	}

	output.Info("✓ Found %d nodes\n", len(nodes))
	for _, node := range nodes {
		output.Info("  - Node: %s (Status: %s)\n", node.Node, node.Status)
	}

	// Try version endpoint
	output.Infoln("Testing version endpoint...")
	version, err := client.GetVersion(ctx)
	if err != nil {
		output.Error("WARNING: Version endpoint not available: %v\n", err)
		output.Infoln("This is common with older Proxmox versions")
	} else {
		output.Info("✓ Proxmox version: %s\n", version.Version)
	}

	// Test cluster resources
	output.Infoln("Testing cluster resources...")
	resources, err := client.GetClusterResources(ctx)
	if err != nil {
		output.Error("Error: Error getting cluster resources: %v\n", err)
		return
	}
	output.Info("✓ Found %d resources\n", len(resources))

	// Count VMs specifically
	vmCount := 0
	for _, resource := range resources {
		if resource.Type == "qemu" {
			vmCount++
		}
	}
	output.Info("✓ Found %d virtual machines\n", vmCount)

	output.Resultln("\n🎉 Connection test successful!")
}
