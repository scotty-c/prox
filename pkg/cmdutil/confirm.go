package cmdutil

import (
	"os"

	"github.com/scotty-c/prox/pkg/output"
)

// ConfirmOrExit asks prompt and reports whether to go ahead. Answering no
// prints cancelled and returns false (the command then exits 0, as the user
// chose not to proceed). If no answer can be read at all, for example in a
// script with stdin closed, it exits 1 naming skipFlag, so a script never
// mistakes an unanswered question for success.
func ConfirmOrExit(prompt, skipFlag, cancelled string) bool {
	ok, err := output.Confirm(prompt, skipFlag)
	if err != nil {
		output.UserError("confirming", err)
		os.Exit(1)
	}
	if !ok {
		output.Errorln(cancelled)
	}
	return ok
}
