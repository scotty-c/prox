package cmd

import (
	"fmt"

	c "github.com/scotty-c/prox/pkg/config"
	"github.com/spf13/cobra"
)

// updateCmd represents the update command to update the config file

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "This command updates the configuration file for prox",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")
		url, _ := cmd.Flags().GetString("url")

		if err := c.Update(username, password, url); err != nil {
			fmt.Printf("Error updating config: %v\n", err)
			return
		}
	},
}

func init() {
	updateCmd.Flags().StringVarP(&username, "username", "u", "", "Username for the Proxmox VE API")
	updateCmd.Flags().StringVarP(&password, "password", "p", "", "Password for the Proxmox VE API")
	updateCmd.Flags().StringVarP(&url, "url", "l", "", "URL for the Proxmox VE API")
	configCmd.AddCommand(updateCmd)
}
