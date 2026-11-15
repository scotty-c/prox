package cmd

import (
	"errors"
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/scotty-c/prox/pkg/completion"
	c "github.com/scotty-c/prox/pkg/config"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

// migrateCmd moves saved credentials to the current storage scheme

var migrateCmd = &cobra.Command{
	Use:   "migrate [profile]",
	Short: "Move saved credentials to secure storage",
	Long: `Move the credentials of saved profiles into the operating system's keychain
(macOS Keychain, Windows Credential Manager, the Linux Secret Service), or,
where there is no keychain, seal them with a random key file.

Profiles saved by older versions of prox encrypted their credentials with a
key derived from your user name and host name. That key can be recomputed by
anything running as you, and renaming the machine makes such profiles
unreadable. Running this once fixes both.

Each profile is read back after it is written; if it does not match, the
profile is left as it was. Profiles that are already protected are skipped.
Versions of prox older than this one cannot read a profile after it has been
moved. 'prox config list' shows how each profile is stored.

Examples:
  prox config migrate            # every profile
  prox config migrate homelab    # one profile`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completion.FirstArgOnly(completion.GetProfileNames),
	RunE: func(cmd *cobra.Command, args []string) error {
		only := ""
		if len(args) == 1 {
			only = args[0]
		}
		results, err := c.UpgradeProfiles(only)
		if err != nil {
			return fmt.Errorf("migrating: %w", err)
		}
		if len(results) == 0 {
			fmt.Println("No profiles found")
			return nil
		}

		t := table.NewWriter()
		t.SetTitle("Credential storage")
		t.AppendHeader(table.Row{"PROFILE", "WAS", "NOW", "RESULT"})
		var failures []error
		for _, r := range results {
			switch {
			case r.Err != nil:
				t.AppendRow(table.Row{r.Profile, string(r.From), string(r.To), "failed"})
				failures = append(failures, fmt.Errorf("%s: %w", r.Profile, r.Err))
			case r.From == r.To:
				t.AppendRow(table.Row{r.Profile, string(r.From), string(r.To), "already secure"})
			default:
				t.AppendRow(table.Row{r.Profile, string(r.From), string(r.To), "moved"})
			}
		}
		t.SetStyle(table.StyleRounded)
		output.Result("%s\n", t.Render())
		if len(failures) > 0 {
			return errors.Join(failures...)
		}
		return nil
	},
}

func init() {
	configCmd.AddCommand(migrateCmd)
}
