package cmd

import (
	"fmt"
	"os"

	c "github.com/scotty-c/prox/pkg/config"
	"github.com/spf13/cobra"
)

// listCmd represents the list command to show all profiles
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configuration profiles",
	Long:  `List all available configuration profiles and indicate which one is currently active.`,
	Run: func(cmd *cobra.Command, args []string) {
		profiles, err := c.ListProfiles()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing profiles: %v\n", err)
			os.Exit(1)
		}

		if len(profiles) == 0 {
			fmt.Println("No profiles found")
			return
		}

		currentProfile := c.GetCurrentProfile()
		fmt.Println("Available profiles:")
		for _, profile := range profiles {
			if profile == currentProfile {
				fmt.Printf("* %s (current)\n", profile)
			} else {
				fmt.Printf("  %s\n", profile)
			}
		}
	},
}

func init() {
	configCmd.AddCommand(listCmd)
}
