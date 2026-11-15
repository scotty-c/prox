package cmdutil

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// MakeGroup turns cmd into a command group: a command that only holds
// subcommands. With no arguments it shows its help; with an argument that is
// not a subcommand (usually a typo) it fails, suggesting close matches,
// instead of quietly printing help and exiting 0.
func MakeGroup(cmd *cobra.Command) {
	cmd.Args = groupArgs
	if cmd.SuggestionsMinimumDistance <= 0 {
		// cobra only applies its default of 2 inside its own lookup
		cmd.SuggestionsMinimumDistance = 2
	}
	cmd.Run = nil
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	}
}

func groupArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	cmd.SilenceUsage = true // the error already says how to get help
	msg := fmt.Sprintf("unknown command %q for %q", args[0], cmd.CommandPath())
	if suggestions := cmd.SuggestionsFor(args[0]); len(suggestions) > 0 {
		msg += "\n\nDid you mean this?\n\t" + strings.Join(suggestions, "\n\t")
	}
	return fmt.Errorf("%s\n\nRun '%s --help' to see the available commands", msg, cmd.CommandPath())
}
