package cmd

import (
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	c "github.com/scotty-c/prox/pkg/config"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

// profileRow is a profile as printed by 'config list -o json'
type profileRow struct {
	Name    string `json:"name"`
	Current bool   `json:"current"`
	URL     string `json:"url,omitempty"`
	User    string `json:"user,omitempty"`
	Error   string `json:"error,omitempty"` // why the profile could not be read
}

// listCmd represents the list command to show all profiles
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List configuration profiles",
	Long: `List the saved configuration profiles with the URL and user each one connects
as, and mark the one in use. Passwords are never shown.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		infos, err := c.ProfileInfos()
		if err != nil {
			return fmt.Errorf("listing profiles: %w", err)
		}

		if output.Structured() {
			rows := make([]profileRow, 0, len(infos))
			for _, in := range infos {
				rows = append(rows, profileRow{Name: in.Name, Current: in.Current, URL: in.URL, User: in.User, Error: in.Error})
			}
			return output.OutputJSON(rows)
		}

		if len(infos) == 0 {
			fmt.Println("No profiles found")
			return nil
		}

		t := table.NewWriter()
		t.SetTitle("Profiles")
		t.AppendHeader(table.Row{"PROFILE", "URL", "USER", "IN USE"})
		for _, in := range infos {
			url, user, inUse := in.URL, in.User, ""
			if in.Error != "" {
				url, user = "(cannot be read)", ""
			}
			if in.Current {
				inUse = "*"
			}
			t.AppendRow(table.Row{in.Name, url, user, inUse})
		}
		t.SetStyle(table.StyleRounded)
		fmt.Println(t.Render())
		for _, in := range infos {
			if in.Error != "" {
				output.Error("Profile '%s' cannot be read: %s\n", in.Name, in.Error)
			}
		}
		return nil
	},
}

func init() {
	configCmd.AddCommand(listCmd)
}
