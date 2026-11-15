package cmd

import (
	"github.com/scotty-c/prox/pkg/container"
	"github.com/spf13/cobra"
)

// shortcutsCmd represents the shortcuts command
var shortcutsCmd = &cobra.Command{
	Use:     "shortcuts",
	Aliases: []string{"shortcut", "sc"},
	Short:   "Show the os:version template shortcuts",
	Long: `Show the os:version shortcuts (such as debian:12) that 'prox ct create' and
'prox ct templates pull' accept instead of a full template name. They come from
the templates your Proxmox node can download, and show which are already
downloaded. If Proxmox cannot be reached, a few common examples are shown.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return container.ListShortcuts(cmd.Context())
	},
}

func init() {
	ctCmd.AddCommand(shortcutsCmd)
}
