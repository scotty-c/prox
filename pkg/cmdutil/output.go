package cmdutil

import (
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

// StructuredOutput reports whether cmd should print JSON or YAML instead of
// a table. A command's own --json flag (kept for compatibility) selects JSON
// and overrides --output.
func StructuredOutput(cmd *cobra.Command) bool {
	if f := cmd.Flags().Lookup("json"); f != nil && f.Changed {
		if jsonFlag, _ := cmd.Flags().GetBool("json"); jsonFlag {
			_ = output.SetFormat(output.FormatJSON)
			return true
		}
	}
	return output.Structured()
}
