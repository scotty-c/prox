package output

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// QuietMode indicates whether to suppress non-essential output
var QuietMode bool

// Info prints informational messages (suppressed in quiet mode)
func Info(format string, a ...interface{}) {
	if !QuietMode {
		fmt.Printf(format, a...)
	}
}

// Infoln prints informational messages with newline (suppressed in quiet mode)
func Infoln(a ...interface{}) {
	if !QuietMode {
		fmt.Println(a...)
	}
}

// Result prints final results (always shown, even in quiet mode)
func Result(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}

// Resultln prints final results with newline (always shown, even in quiet mode)
func Resultln(a ...interface{}) {
	fmt.Println(a...)
}

// Error prints error messages to stderr (always shown)
func Error(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}

// Errorln prints error messages to stderr with newline (always shown)
func Errorln(a ...interface{}) {
	fmt.Fprintln(os.Stderr, a...)
}

// SetQuiet enables or disables quiet mode
func SetQuiet(quiet bool) {
	QuietMode = quiet
}

// IsQuiet returns whether quiet mode is enabled
func IsQuiet() bool {
	return QuietMode
}

// Confirm prompts the user for confirmation before a destructive action
// Returns true if the user confirms (y/yes), false otherwise
// The prompt should be phrased as a yes/no question
func Confirm(prompt string) bool {
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", prompt)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}
