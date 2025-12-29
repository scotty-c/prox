package cmd

import (
	"fmt"

	c "github.com/scotty-c/prox/pkg/config"
	"github.com/spf13/cobra"
)

// useCmd represents the use command to switch active profile
var useCmd = &cobra.Command{
	Use:   "use <profile>",
	Short: "Switch to a different configuration profile",
	Long:  `Switch the active configuration profile. The specified profile must already exist.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		profile := args[0]

		if err := c.SetCurrentProfile(profile); err != nil {
			fmt.Printf("Error switching profile: %v\n", err)
			return
		}

		fmt.Printf("Switched to profile: %s\n", profile)
	},
}

func init() {
	configCmd.AddCommand(useCmd)
}
