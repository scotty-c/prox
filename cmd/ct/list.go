package cmd

import (
	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/container"
	"github.com/spf13/cobra"
)

// listCmd represents the list command for containers
var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List LXC containers",
	Long:    `List all LXC containers on Proxmox VE with their status, resource usage, and node information.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		node, _ := cmd.Flags().GetString("node")
		running, _ := cmd.Flags().GetBool("running")
		jsonOutput := cmdutil.StructuredOutput(cmd)
		tag, _ := cmd.Flags().GetString("tag")
		if err := container.ListContainers(cmd.Context(), node, running, jsonOutput, tag); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	listCmd.Flags().StringP("node", "n", "", "Proxmox node name (optional - will show containers from all nodes if not specified)")
	listCmd.Flags().BoolP("running", "r", false, "Show only running containers")
	listCmd.Flags().Bool("json", false, "Output as JSON (same as -o json)")
	listCmd.Flags().StringP("tag", "t", "", "Filter by tag (e.g., 'production', 'database')")
	ctCmd.AddCommand(listCmd)
	completion.RegisterNodeFlag(listCmd, "node")
}
