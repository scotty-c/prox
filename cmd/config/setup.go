package cmd

import (
	"fmt"
	"os"

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
	Long: `Save connection details for the current profile.

Certificates are verified by default. If your Proxmox server uses its default
self-signed certificate, add --ca-cert with a copy of /etc/pve/pve-root-ca.pem
from a node, or --insecure to skip verification.

Example:
  prox config setup -u root@pam -p secret -l https://pve.example.com:8006 --ca-cert ~/pve-root-ca.pem`,
	Run: func(cmd *cobra.Command, args []string) {
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")
		url, _ := cmd.Flags().GetString("url")

		if c.Check() {
			fmt.Printf("Config already exists for profile '%s'. Use 'prox config update' to change it\n", c.GetCurrentProfile())
			saveTLSFlagsOrExit(cmd, c.GetCurrentProfile())
			return
		}

		if err := c.Create(username, password, url); err != nil {
			fmt.Fprintf(os.Stderr, "Error setting up config: %v\n", err)
			os.Exit(1)
		}
		saveTLSFlagsOrExit(cmd, c.GetCurrentProfile())
		fmt.Println("Config setup completed successfully")
	},
}

func init() {
	setupCmd.Flags().StringVarP(&username, "username", "u", "", "Username for the Proxmox VE API")
	setupCmd.Flags().StringVarP(&password, "password", "p", "", "Password for the Proxmox VE API")
	setupCmd.Flags().StringVarP(&url, "url", "l", "", "URL for the Proxmox VE API")
	configCmd.AddCommand(setupCmd)
}
