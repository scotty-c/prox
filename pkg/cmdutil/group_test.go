package cmdutil

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newTree() *cobra.Command {
	root := &cobra.Command{Use: "prox"}
	group := &cobra.Command{Use: "vm", Short: "group"}
	MakeGroup(group)
	group.AddCommand(&cobra.Command{Use: "list", Aliases: []string{"ls"}, Run: func(*cobra.Command, []string) {}})
	root.AddCommand(group)
	root.SilenceUsage, root.SilenceErrors = true, true
	return root
}

func run(args ...string) (string, error) {
	root := newTree()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestGroupWithNoArgsShowsHelp(t *testing.T) {
	out, err := run("vm")
	if err != nil || !strings.Contains(out, "Available Commands") {
		t.Fatalf("want help and no error, got err=%v out=%q", err, out)
	}
}

func TestGroupRejectsUnknownSubcommand(t *testing.T) {
	_, err := run("vm", "bogus")
	if err == nil || !strings.Contains(err.Error(), `unknown command "bogus" for "prox vm"`) {
		t.Fatalf("got %v", err)
	}
}

func TestGroupSuggestsCloseMatches(t *testing.T) {
	_, err := run("vm", "lst")
	if err == nil || !strings.Contains(err.Error(), "Did you mean this?") || !strings.Contains(err.Error(), "list") {
		t.Fatalf("got %v", err)
	}
}

func TestGroupStillRunsSubcommands(t *testing.T) {
	if _, err := run("vm", "ls"); err != nil {
		t.Fatal(err)
	}
}
