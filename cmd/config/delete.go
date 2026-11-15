package cmd

import (
	"fmt"

	c "github.com/scotty-c/prox/pkg/config"
	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command to remove the config file

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "This command deletes the configuration file for prox",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := c.Delete(); err != nil {
			return fmt.Errorf("deleting config: %w", err)
		}
		return nil
	},
}

func init() {
	configCmd.AddCommand(deleteCmd)
}
