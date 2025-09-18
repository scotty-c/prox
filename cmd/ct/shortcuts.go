package cmd

import (
	"github.com/scotty-c/prox/pkg/container"
	"github.com/spf13/cobra"
)

// shortcutsCmd represents the shortcuts command
var shortcutsCmd = &cobra.Command{
	Use:     "shortcuts",
	Aliases: []string{"shortcut", "sc"},
	Short:   "Show common template shortcuts",
	Long:    `Display common template shortcuts that can be used with the create command instead of full template paths.`,
	Run: func(cmd *cobra.Command, args []string) {
		container.ListTemplateShortcuts()
	},
}

func init() {
	ctCmd.AddCommand(shortcutsCmd)
}
