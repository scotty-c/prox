package cmdutil

import "github.com/spf13/cobra"

// AddYesFlag registers -y/--yes, which skips a confirmation prompt. Commands
// that already had --force for this keep it, so existing scripts still work;
// use SkipConfirm to read either.
func AddYesFlag(cmd *cobra.Command) {
	cmd.Flags().BoolP("yes", "y", false, "Don't ask for confirmation")
}

// SkipConfirm reports whether the user asked to skip the confirmation prompt
// with --yes or --force
func SkipConfirm(cmd *cobra.Command) bool {
	yes, _ := cmd.Flags().GetBool("yes")
	if f := cmd.Flags().Lookup("force"); f != nil {
		force, _ := cmd.Flags().GetBool("force")
		return yes || force
	}
	return yes
}
