package cmd

import (
	"fmt"

	c "github.com/scotty-c/prox/pkg/config"
	"github.com/spf13/cobra"
)

// migrateCmd represents the migrate command to upgrade config to encrypted format

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "This command migrates an existing plain text config to encrypted format",
	Long:  `This command will detect if your configuration file contains plain text credentials and migrate them to encrypted format for improved security.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := c.MigrateConfig(); err != nil {
			fmt.Printf("Error migrating config: %v\n", err)
			return
		}
		fmt.Println("Config migration completed successfully")
	},
}

func init() {
	configCmd.AddCommand(migrateCmd)
}
