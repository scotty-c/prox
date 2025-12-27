package cmd

import (
	"context"
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/config"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show connection status and cluster health",
	Long: `Display connection status and cluster health information.

This command will:
1. Verify connection to the Proxmox VE cluster
2. Show cluster name and version
3. Display node count and status
4. Show VM and container counts with status breakdown

Examples:
  prox status                    # Show cluster status
  prox status --quiet            # Show minimal output`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return showStatus()
	},
}

func init() {
	RootCmd.AddCommand(statusCmd)
}

func showStatus() error {
	// Check if config exists
	if !config.Check() {
		return fmt.Errorf("no configuration found. Run 'prox config setup' to configure connection")
	}

	// Read config
	_, _, url, err := config.Read()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	fmt.Println("Connecting to Proxmox VE cluster...")

	// Create client
	cl, err := client.CreateClient()
	if err != nil {
		output.ClientError(err)
		return fmt.Errorf("failed to connect to Proxmox VE: %w", err)
	}

	ctx := context.Background()

	// Get version info
	version, err := cl.GetVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get cluster version: %w", err)
	}

	// Get nodes
	nodes, err := cl.GetNodes(ctx)
	if err != nil {
		return fmt.Errorf("failed to get cluster nodes: %w", err)
	}

	// Get cluster resources
	resources, err := cl.GetClusterResources(ctx)
	if err != nil {
		return fmt.Errorf("failed to get cluster resources: %w", err)
	}

	// Count resources by type and status
	vmCounts := make(map[string]int)
	ctCounts := make(map[string]int)
	totalVMs := 0
	totalCTs := 0

	for _, resource := range resources {
		switch resource.Type {
		case "qemu":
			totalVMs++
			vmCounts[resource.Status]++
		case "lxc":
			totalCTs++
			ctCounts[resource.Status]++
		}
	}

	// Count nodes by status
	onlineNodes := 0
	offlineNodes := 0
	for _, node := range nodes {
		if node.Status == "online" {
			onlineNodes++
		} else {
			offlineNodes++
		}
	}

	// Display connection information
	fmt.Printf("\n Connection Status\n")
	fmt.Printf("╭─────────────────────────────────────────────────────────╮\n")
	fmt.Printf("│ URL:       %-45s│\n", url)
	fmt.Printf("│ Version:   %-45s│\n", version.Version)
	fmt.Printf("│ Release:   %-45s│\n", version.Release)
	fmt.Printf("╰─────────────────────────────────────────────────────────╯\n")

	// Display cluster health
	fmt.Printf("\n Cluster Health\n")
	t := table.NewWriter()
	t.AppendHeader(table.Row{"METRIC", "COUNT", "DETAILS"})

	// Nodes
	nodeDetails := fmt.Sprintf("%d online", onlineNodes)
	if offlineNodes > 0 {
		nodeDetails = fmt.Sprintf("%d online, %d offline", onlineNodes, offlineNodes)
	}
	t.AppendRow(table.Row{"Nodes", len(nodes), nodeDetails})

	// VMs
	vmDetails := ""
	if vmCounts["running"] > 0 || vmCounts["stopped"] > 0 {
		vmDetails = fmt.Sprintf("%d running, %d stopped", vmCounts["running"], vmCounts["stopped"])
		if vmCounts["paused"] > 0 {
			vmDetails += fmt.Sprintf(", %d paused", vmCounts["paused"])
		}
	}
	t.AppendRow(table.Row{"Virtual Machines", totalVMs, vmDetails})

	// Containers
	ctDetails := ""
	if ctCounts["running"] > 0 || ctCounts["stopped"] > 0 {
		ctDetails = fmt.Sprintf("%d running, %d stopped", ctCounts["running"], ctCounts["stopped"])
		if ctCounts["paused"] > 0 {
			ctDetails += fmt.Sprintf(", %d paused", ctCounts["paused"])
		}
	}
	t.AppendRow(table.Row{"LXC Containers", totalCTs, ctDetails})

	t.SetStyle(table.StyleRounded)
	fmt.Printf("%s\n", t.Render())

	// Summary status
	fmt.Printf("\nStatus: ")
	if offlineNodes > 0 {
		fmt.Printf("⚠️  DEGRADED - %d node(s) offline\n", offlineNodes)
	} else {
		fmt.Printf("✓ HEALTHY - All systems operational\n")
	}

	return nil
}
