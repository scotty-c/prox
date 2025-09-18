package cmd

import (
	"github.com/scotty-c/prox/pkg/container"
	"github.com/spf13/cobra"
)

// templatesCmd represents the templates command
var templatesCmd = &cobra.Command{
	Use:     "templates",
	Aliases: []string{"template", "tmpl"},
	Short:   "List available container templates",
	Long:    `List all available LXC container templates on Proxmox VE nodes. This shows the built-in templates that can be used to create new containers.`,
	Run: func(cmd *cobra.Command, args []string) {
		node, _ := cmd.Flags().GetString("node")
		container.ListTemplates(node)
	},
}

func init() {
	templatesCmd.Flags().StringP("node", "n", "", "Proxmox node name (optional - will show templates from all nodes if not specified)")
	ctCmd.AddCommand(templatesCmd)
}
