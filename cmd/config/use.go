package cmd

import (
	"fmt"
	"github.com/scotty-c/prox/pkg/completion"

	c "github.com/scotty-c/prox/pkg/config"
	"github.com/spf13/cobra"
)

// useCmd represents the use command to switch active profile
var useCmd = &cobra.Command{
	Use:               "use <profile>",
	Short:             "Switch to a different configuration profile",
	Long:              `Switch the active configuration profile. The specified profile must already exist.`,
	ValidArgsFunction: completion.FirstArgOnly(completion.GetProfileNames),
	Args:              cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profile := args[0]

		if err := c.SetCurrentProfile(profile); err != nil {
			return fmt.Errorf("switching profile: %w", err)
		}

		fmt.Printf("Switched to profile: %s\n", profile)
		return nil
	},
}

func init() {
	configCmd.AddCommand(useCmd)
}
