package cmd

import (
	"fmt"

	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/completion"
	c "github.com/scotty-c/prox/pkg/config"
	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command to remove a profile

var deleteCmd = &cobra.Command{
	Use:   "delete [profile]",
	Short: "Delete a configuration profile and its saved credentials",
	Long: `Delete a configuration profile and the credentials saved in it. Without a
profile name, the current profile is deleted. You are asked to confirm first
unless --force is given.

Examples:
  prox config delete homelab
  prox config delete --yes         # the current profile, without asking`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completion.FirstArgOnly(completion.GetProfileNames),
	RunE: func(cmd *cobra.Command, args []string) error {
		profile := c.GetCurrentProfile()
		if len(args) == 1 {
			profile = args[0]
		}
		if !c.ProfileExists(profile) {
			return fmt.Errorf("profile '%s' does not exist (see 'prox config list')", profile)
		}

		if !cmdutil.SkipConfirm(cmd) {
			ok, err := cmdutil.Confirm(
				fmt.Sprintf("Delete profile '%s'? Its saved credentials will be removed", profile),
				"--yes", "Deletion cancelled")
			if err != nil || !ok {
				return err
			}
		}

		if err := c.DeleteProfile(profile); err != nil {
			return fmt.Errorf("deleting profile: %w", err)
		}
		fmt.Printf("Profile '%s' deleted\n", profile)
		return nil
	},
}

func init() {
	deleteCmd.Flags().BoolP("force", "f", false, "Don't ask for confirmation (same as --yes)")
	cmdutil.AddYesFlag(deleteCmd)
	configCmd.AddCommand(deleteCmd)
}
