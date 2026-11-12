package cmdutil

import (
	"fmt"

	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

// AddTagFlags registers --tag (act on every resource with a tag instead of a
// single name) and -y/--yes (skip the confirmation for --tag)
func AddTagFlags(cmd *cobra.Command, kind string) {
	cmd.Flags().StringP("tag", "t", "", fmt.Sprintf("Act on every %s with this tag instead of a single name", kind))
	cmd.Flags().BoolP("yes", "y", false, "With --tag, don't ask for confirmation")
}

// NameOrTagArgs accepts exactly one name, or none when --tag is given
func NameOrTagArgs(cmd *cobra.Command, args []string) error {
	tag, _ := cmd.Flags().GetString("tag")
	switch {
	case tag != "" && len(args) > 0:
		return fmt.Errorf("give either a name or --tag, not both")
	case tag == "" && len(args) != 1:
		return fmt.Errorf("requires exactly 1 argument: <name|id> (or use --tag)")
	}
	return nil
}

// BulkTarget is one resource in a bulk operation
type BulkTarget struct {
	Label string // e.g. "web-1 (ID: 101) on pve1"
	Run   func() error
}

// RunBulk lists the targets, asks for confirmation unless skipConfirm, then
// runs each one. It keeps going after a failure and returns an error if any
// failed. verb describes the action, e.g. "start".
func RunBulk(verb, kind, tag string, targets []BulkTarget, skipConfirm bool) error {
	if len(targets) == 0 {
		return fmt.Errorf("no %ss have the tag '%s'", kind, tag)
	}

	output.Error("This will %s %d %s(s) tagged '%s':\n", verb, len(targets), kind, tag)
	for _, t := range targets {
		output.Error("  • %s\n", t.Label)
	}
	if !skipConfirm && !output.Confirm("Continue?") {
		output.Errorln("Cancelled")
		return nil
	}

	failed := 0
	for _, t := range targets {
		if err := t.Run(); err != nil {
			output.Error("Error: %s: %v\n", t.Label, err)
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d %s(s) failed to %s", failed, len(targets), kind, verb)
	}
	return nil
}
