package node

import (
	"context"
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

// listCmd represents the 'ls' command for nodes
var listCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List all Proxmox nodes in the datacenter",
	RunE: func(cmd *cobra.Command, args []string) error {
		structured := cmdutil.StructuredOutput(cmd)

		c, err := client.CreateClient()
		if err != nil {
			return fmt.Errorf("Failed to create client: %w", err)
		}

		nodes, err := c.GetNodes(context.Background())
		if err != nil {
			return fmt.Errorf("Failed to list nodes: %w", err)
		}

		if structured {
			if nodes == nil {
				nodes = []client.Node{} // print [] rather than null
			}
			if err := output.OutputJSON(nodes); err != nil {
				return fmt.Errorf("encoding output: %w", err)
			}
			return nil
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
		return nil
	},
}

func init() {
	listCmd.Flags().Bool("json", false, "Output as JSON (same as -o json)")
	NodeCmd.AddCommand(listCmd)
}
