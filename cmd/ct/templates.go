package cmd

import (
	"os"

	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/container"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

// templatesCmd represents the templates command
var templatesCmd = &cobra.Command{
	Use:     "templates",
	Aliases: []string{"template", "tmpl"},
	Short:   "List available container templates",
	Long: `List the LXC container templates downloaded on your Proxmox VE nodes, which can
be used to create containers. Use --available to see templates Proxmox can
download, and 'prox ct templates pull' to download one.

Examples:
  prox ct templates                    # Downloaded templates
  prox ct templates --available        # Templates you can download
  prox ct templates pull debian:12     # Download a template`,
	Run: func(cmd *cobra.Command, args []string) {
		node, _ := cmd.Flags().GetString("node")
		available, _ := cmd.Flags().GetBool("available")
		if available {
			if err := container.ListAvailableTemplates(node); err != nil {
				output.UserError("listing downloadable templates", err)
				os.Exit(1)
			}
			return
		}
		if err := container.ListTemplates(node); err != nil {
			os.Exit(1)
		}
	},
}

var pullCmd = &cobra.Command{
	Use:     "pull <os:version|template>",
	Aliases: []string{"download"},
	Short:   "Download a container template",
	Long: `Download a container template from the Proxmox template repository.

Give an os:version shortcut (the same ones 'prox ct create' accepts) or a full
template name from 'prox ct templates --available'. Without --node, the first
online node is used; without --storage, 'local' is used if it can hold templates.

Examples:
  prox ct templates pull debian:12
  prox ct templates pull ubuntu:24.04 --node pve2 --storage nfs-templates`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		node, _ := cmd.Flags().GetString("node")
		storage, _ := cmd.Flags().GetString("storage")
		wait := cmdutil.ResolveWait(cmd, true)
		if err := container.PullTemplate(args[0], node, storage, wait); err != nil {
			output.UserError("downloading template", err)
			os.Exit(1)
		}
	},
}

func init() {
	templatesCmd.Flags().StringP("node", "n", "", "Proxmox node name (optional - will show templates from all nodes if not specified)")
	templatesCmd.Flags().Bool("available", false, "List templates that can be downloaded instead of downloaded ones")
	ctCmd.AddCommand(templatesCmd)
	completion.RegisterNodeFlag(templatesCmd, "node")

	pullCmd.Flags().StringP("node", "n", "", "Node to download to (default: first online node)")
	pullCmd.Flags().StringP("storage", "s", "", "Storage to download to (default: local, or the first storage that can hold templates)")
	cmdutil.AddWaitFlags(pullCmd, true)
	templatesCmd.AddCommand(pullCmd)
	completion.RegisterNodeFlag(pullCmd, "node")
}
