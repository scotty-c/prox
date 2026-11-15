package output

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/scotty-c/prox/pkg/apierr"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

// QuietMode indicates whether to suppress non-essential output
var QuietMode bool

// Info prints informational messages (suppressed in quiet mode, and with
// --output json/yaml so stdout stays machine-readable)
func Info(format string, a ...interface{}) {
	if !QuietMode && !Structured() {
		fmt.Printf(format, a...)
	}
}

// Infoln prints informational messages with newline (suppressed in quiet
// mode, and with --output json/yaml so stdout stays machine-readable)
func Infoln(a ...interface{}) {
	if !QuietMode && !Structured() {
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

// StartSpinner shows a progress spinner on stderr and returns a function that
// stops it. Nothing is shown in quiet mode or when stderr is not a terminal,
// so piped and scripted output stays clean.
func StartSpinner(suffix string) func() {
	if QuietMode || !term.IsTerminal(int(os.Stderr.Fd())) {
		return func() {}
	}
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(os.Stderr))
	s.Suffix = suffix
	s.Start()
	return s.Stop
}

// SetQuiet enables or disables quiet mode
func SetQuiet(quiet bool) {
	QuietMode = quiet
}

// Output formats for --output
const (
	FormatTable = "table"
	FormatJSON  = "json"
	FormatYAML  = "yaml"
)

// format is the --output format for this run
var format = FormatTable

// SetFormat sets the output format from --output
func SetFormat(f string) error {
	switch f {
	case FormatTable, FormatJSON, FormatYAML:
		format = f
		return nil
	case "":
		format = FormatTable
		return nil
	}
	return fmt.Errorf("invalid --output %q: use table, json or yaml", f)
}

// Format returns the output format for this run
func Format() string {
	return format
}

// Structured reports whether output should be JSON or YAML instead of tables
func Structured() bool {
	return format == FormatJSON || format == FormatYAML
}

// OutputJSON prints data to stdout as JSON, or as YAML with --output yaml.
// YAML uses the same field names as JSON.
func OutputJSON(data interface{}) error {
	if format == FormatYAML {
		// Convert via JSON so YAML keys match the JSON field names
		raw, err := json.Marshal(data)
		if err != nil {
			return err
		}
		var generic interface{}
		if err := json.Unmarshal(raw, &generic); err != nil {
			return err
		}
		enc := yaml.NewEncoder(os.Stdout)
		enc.SetIndent(2)
		if err := enc.Encode(generic); err != nil {
			return err
		}
		return enc.Close()
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// Confirm asks a yes/no question on stderr and reads the answer from stdin.
// It returns true for y/yes and false for anything else that was typed.
//
// If no answer can be read at all (stdin is closed, e.g. </dev/null or a CI
// job), it returns an error rather than false: treating that as "no" would let
// a script carry on as if the command had succeeded. skipFlag names the flag
// that skips the question, e.g. "--force", and is used in that error.
func Confirm(prompt, skipFlag string) (bool, error) {
	return confirmFrom(os.Stdin, os.Stderr, prompt, skipFlag)
}

func confirmFrom(in io.Reader, out io.Writer, prompt, skipFlag string) (bool, error) {
	fmt.Fprintf(out, "%s [y/N]: ", prompt)

	response, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && strings.TrimSpace(response) == "" {
		fmt.Fprintln(out)
		return false, fmt.Errorf("no answer to the confirmation prompt (stdin is closed or not a terminal); pass %s to skip it", skipFlag)
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes", nil
}

// Fail prints a command's error to stderr, followed by a tip when the kind of
// error is recognised. Commands return their errors and cmd.Execute calls
// this once, so an error is never printed twice.
func Fail(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	if tip := tipFor(err); tip != "" {
		fmt.Fprintf(os.Stderr, "\nTip: %s\n", tip)
	}
}

// tipFor suggests what to do about err, or "" if there is nothing useful to
// add (including when the error text already explains what to do)
func tipFor(err error) string {
	switch {
	case errors.Is(err, apierr.ErrCertificate):
		return "" // the message already lists the options
	case errors.Is(err, apierr.ErrNoConfig):
		return "Run 'prox config setup' to save your connection, or set PROX_URL, PROX_USER and PROX_PASSWORD"
	case errors.Is(err, apierr.ErrUnreachable):
		return "Check the URL with 'prox config read', that the server is up, and that port 8006 is not blocked"
	case errors.Is(err, apierr.ErrAuth):
		return "Check your credentials with 'prox config test'. For an API token, use the token ID (user@realm!name) as the username and the secret as the password"
	case errors.Is(err, apierr.ErrPermission):
		return "Your Proxmox user lacks the privileges this needs; check its role and permissions"
	}
	return ""
}
