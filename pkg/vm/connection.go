package vm

import (
	"context"
	"fmt"

	c "github.com/scotty-c/prox/pkg/client"
)

// Status gets the status/IP of a virtual machine
func (v *VirtualMachine) Status(ctx context.Context, id int, targetNode string) (string, error) {
	return v.Client.GetVMIP(ctx, targetNode, id)
}

// GetIp gets the IP address of a VM by ID and node
func GetIp(id int, node string) string {
	client, err := c.CreateClient()
	if err != nil {
		return "Error getting IP"
	}

	ip, err := client.GetVMIP(context.Background(), node, id)
	if err != nil {
		return "Error getting IP"
	}

	return ip
}

// TestConnection tests the basic connection to Proxmox
func TestConnection() {
	client, err := c.CreateClient()
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return
	}

	fmt.Println("Testing connection to Proxmox server...")

	// Try nodes first (more compatible with older versions)
	fmt.Println("Testing nodes endpoint...")
	nodes, err := client.GetNodes(context.Background())
	if err != nil {
		fmt.Printf("❌ Error getting nodes: %v\n", err)

		// Try version endpoint as fallback
		fmt.Println("Trying version endpoint...")
		version, err2 := client.GetVersion(context.Background())
		if err2 != nil {
			fmt.Printf("❌ Error getting version: %v\n", err2)
			fmt.Println("\nBoth nodes and version endpoints failed.")
			fmt.Println("This suggests an authentication issue or API incompatibility.")
			fmt.Println("\nTroubleshooting tips:")
			fmt.Println("1. Verify credentials are correct")
			fmt.Println("2. Check if API access is enabled for the user")
			fmt.Println("3. This might be an older Proxmox version with limited API support")
			return
		}
		fmt.Printf("✓ Connected to Proxmox version: %s\n", version.Version)
		fmt.Println("⚠️  Nodes endpoint not available, but version endpoint works")
		return
	}

	fmt.Printf("✓ Found %d nodes\n", len(nodes))
	for _, node := range nodes {
		fmt.Printf("  - Node: %s (Status: %s)\n", node.Node, node.Status)
	}

	// Try version endpoint
	fmt.Println("Testing version endpoint...")
	version, err := client.GetVersion(context.Background())
	if err != nil {
		fmt.Printf("⚠️  Version endpoint not available: %v\n", err)
		fmt.Println("This is common with older Proxmox versions")
	} else {
		fmt.Printf("✓ Proxmox version: %s\n", version.Version)
	}

	// Test cluster resources
	fmt.Println("Testing cluster resources...")
	resources, err := client.GetClusterResources(context.Background())
	if err != nil {
		fmt.Printf("❌ Error getting cluster resources: %v\n", err)
		return
	}
	fmt.Printf("✓ Found %d resources\n", len(resources))

	// Count VMs specifically
	vmCount := 0
	for _, resource := range resources {
		if resource.Type == "qemu" {
			vmCount++
		}
	}
	fmt.Printf("✓ Found %d virtual machines\n", vmCount)

	fmt.Println("\n🎉 Connection test successful!")
}
