package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// removedCommand stands in for a command that no longer exists, so a script
// that still calls it gets an explanation rather than "unknown command"
func removedCommand(name, instead string) *cobra.Command {
	return &cobra.Command{
		Use:                name,
		Hidden:             true,
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("'prox config %s' was removed in 1.0: use %s, which creates the profile or changes it, and checks the connection first", name, instead)
		},
	}
}

func init() {
	configCmd.AddCommand(
		removedCommand("create", "'prox config setup <profile>'"),
		removedCommand("update", "'prox config setup' (it changes the current profile, or the one you name)"),
	)
}
