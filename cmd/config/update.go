package cmd

import (
	"fmt"

	c "github.com/scotty-c/prox/pkg/config"
	"github.com/spf13/cobra"
)

// updateCmd represents the update command to update the config file

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Change details saved in the current profile",
	Long: `Change the URL, user, password or TLS settings saved in the current profile.
Only what you give is changed.

Examples:
  prox config update --url https://pve2.example.com:8006
  echo "$TOKEN_SECRET" | prox config update -u 'ci@pve!deploy' --password-stdin
  prox config update --ca-cert ~/pve-root-ca.pem`,
	RunE: func(cmd *cobra.Command, args []string) error {
		username, password, url, err := readCredentialFlags(cmd)
		if err != nil {
			return err
		}

		if err := saveTLSFlags(cmd, c.GetCurrentProfile()); err != nil {
			return err
		}
		if err := c.Update(username, password, url); err != nil {
			return fmt.Errorf("updating config: %w", err)
		}
		return nil
	},
}

func init() {
	addCredentialFlags(updateCmd)
	configCmd.AddCommand(updateCmd)
}
