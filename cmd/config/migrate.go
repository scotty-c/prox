package cmd

import (
	"fmt"

	c "github.com/scotty-c/prox/pkg/config"
	"github.com/spf13/cobra"
)

// migrateCmd represents the migrate command to upgrade config to encrypted format

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Encrypt credentials saved in an old plain-text format",
	Long:  `This command will detect if your configuration file contains plain text credentials and migrate them to encrypted format for improved security.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := c.MigrateConfig(); err != nil {
			return fmt.Errorf("migrating config: %w", err)
		}
		fmt.Println("Config migration completed successfully")
		return nil
	},
}

func init() {
	configCmd.AddCommand(migrateCmd)
}
