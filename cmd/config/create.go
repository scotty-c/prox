package cmd

import (
	"fmt"

	c "github.com/scotty-c/prox/pkg/config"
	"github.com/spf13/cobra"
)

// createCmd represents the create command to create a new profile
var createCmd = &cobra.Command{
	Use:   "create <profile>",
	Short: "Create a new configuration profile",
	Long:  `Create a new configuration profile with the specified name and credentials.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		profile := args[0]
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")
		url, _ := cmd.Flags().GetString("url")

		if err := c.CreateProfile(profile, username, password, url); err != nil {
			fmt.Printf("Error creating profile: %v\n", err)
			return
		}

		fmt.Printf("Profile '%s' created successfully\n", profile)

		// Ask if they want to switch to this profile
		switchToNew, _ := cmd.Flags().GetBool("use")
		if switchToNew {
			if err := c.SetCurrentProfile(profile); err != nil {
				fmt.Printf("Warning: profile created but failed to switch: %v\n", err)
			} else {
				fmt.Printf("Switched to profile: %s\n", profile)
			}
		}
	},
}

func init() {
	createCmd.Flags().StringP("username", "u", "", "Username for the Proxmox VE API")
	createCmd.Flags().StringP("password", "p", "", "Password for the Proxmox VE API")
	createCmd.Flags().StringP("url", "l", "", "URL for the Proxmox VE API")
	createCmd.Flags().Bool("use", false, "Switch to this profile after creation")
	configCmd.AddCommand(createCmd)
}
