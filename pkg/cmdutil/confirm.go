package cmdutil

import (
	"github.com/scotty-c/prox/pkg/output"
)

// Confirm asks prompt and reports whether to go ahead. Answering no prints
// cancelled and returns false with no error (the command then ends normally,
// as the user chose not to proceed). If no answer can be read at all, for
// example in a script with stdin closed, it returns an error naming skipFlag,
// so a script never mistakes an unanswered question for success.
func Confirm(prompt, skipFlag, cancelled string) (bool, error) {
	ok, err := output.Confirm(prompt, skipFlag)
	if err != nil {
		return false, err
	}
	if !ok {
		output.Errorln(cancelled)
	}
	return ok, nil
}
