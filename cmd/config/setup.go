package cmd

import (
	"fmt"

	c "github.com/scotty-c/prox/pkg/config"
	"github.com/spf13/cobra"
)

// setupCmd represents the setup command

var username string
var password string
var url string

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "This command sets up the configuration file for prox",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")
		url, _ := cmd.Flags().GetString("url")

		if err := c.FirstRun(username, password, url); err != nil {
			fmt.Printf("Error setting up config: %v\n", err)
			return
		}
		fmt.Println("Config setup completed successfully")
	},
}

func init() {
	setupCmd.Flags().StringVarP(&username, "username", "u", "", "Username for the Proxmox VE API")
	setupCmd.Flags().StringVarP(&password, "password", "p", "", "Password for the Proxmox VE API")
	setupCmd.Flags().StringVarP(&url, "url", "l", "", "URL for the Proxmox VE API")
	configCmd.AddCommand(setupCmd)
}
