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
	Run: func(cmd *cobra.Command, args []string) {
		if err := c.Delete(); err != nil {
			fmt.Printf("Error deleting config: %v\n", err)
			return
		}
	},
}

func init() {
	configCmd.AddCommand(deleteCmd)
}
