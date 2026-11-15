package cmd

import (
	"fmt"

	c "github.com/scotty-c/prox/pkg/config"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

// listCmd represents the list command to show all profiles
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configuration profiles",
	Long:  `List all available configuration profiles and indicate which one is currently active.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles, err := c.ListProfiles()
		if err != nil {
			return fmt.Errorf("listing profiles: %w", err)
		}

		currentProfile := c.GetCurrentProfile()

		if output.Structured() {
			type profileInfo struct {
				Name    string `json:"name"`
				Current bool   `json:"current"`
			}
			list := []profileInfo{}
			for _, profile := range profiles {
				list = append(list, profileInfo{Name: profile, Current: profile == currentProfile})
			}
			if err := output.OutputJSON(list); err != nil {
				return fmt.Errorf("encoding output: %w", err)
			}
			return nil
		}

		if len(profiles) == 0 {
			fmt.Println("No profiles found")
			return nil
		}

		fmt.Println("Available profiles:")
		for _, profile := range profiles {
			if profile == currentProfile {
				fmt.Printf("* %s (current)\n", profile)
			} else {
				fmt.Printf("  %s\n", profile)
			}
		}
		return nil
	},
}

func init() {
	configCmd.AddCommand(listCmd)
}
