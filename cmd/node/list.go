package node

import (
	"context"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/scotty-c/prox/pkg/client"
	"github.com/spf13/cobra"
)

// listCmd represents the 'ls' command for nodes
var listCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all Proxmox nodes in the datacenter",
	Run: func(cmd *cobra.Command, args []string) {
		c, err := client.CreateClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create client: %v\n", err)
			os.Exit(1)
		}

		nodes, err := c.GetNodes(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list nodes: %v\n", err)
			os.Exit(1)
		}

		// Display nodes in a formatted table to match prox vm ls
		t := table.NewWriter()
		t.SetTitle("Proxmox Nodes")
		t.AppendHeader(table.Row{"NAME", "STATUS", "TYPE", "ID"})

		for _, n := range nodes {
			t.AppendRow(table.Row{n.Node, n.Status, n.Type, n.ID})
		}

		t.SetStyle(table.StyleRounded)
		fmt.Printf("\n%s\n", t.Render())
		fmt.Printf("Found %d node(s)\n", len(nodes))
	},
}

func init() {
	NodeCmd.AddCommand(listCmd)
}
