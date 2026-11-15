package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	c "github.com/scotty-c/prox/pkg/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// createCmd represents the create command to create a new profile
var createCmd = &cobra.Command{
	Use:   "create <profile>",
	Short: "Create a new configuration profile",
	Long:  `Create a new configuration profile with the specified name and credentials.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profile := args[0]
		username, password, url, err := readCredentialFlags(cmd)
		if err != nil {
			return err
		}

		// On a terminal, ask for anything not given as a flag, as setup does
		if term.IsTerminal(int(os.Stdin.Fd())) {
			if username, password, url, err = promptMissing(username, password, url); err != nil {
				return fmt.Errorf("reading input: %w", err)
			}
			// what was typed at the prompt gets the same tidying as a flag
			if url, _, err = c.NormalizeURL(url); err != nil {
				return err
			}
		}
		// A profile without these cannot connect, so don't save one
		var missing []string
		for flag, value := range map[string]string{"--url": url, "--username": username, "--password": password} {
			if strings.TrimSpace(value) == "" {
				missing = append(missing, flag)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			return fmt.Errorf("a profile needs a URL, username and password; missing %s (run on a terminal to be prompted)", strings.Join(missing, ", "))
		}

		if err := c.CreateProfile(profile, username, password, url); err != nil {
			return fmt.Errorf("creating profile: %w", err)
		}

		if err := saveTLSFlags(cmd, profile); err != nil {
			return err
		}
		fmt.Printf("Profile '%s' created successfully\n", profile)

		// Ask if they want to switch to this profile
		switchToNew, _ := cmd.Flags().GetBool("use")
		if switchToNew {
			if err := c.SetCurrentProfile(profile); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: profile created but failed to switch: %v\n", err)
			} else {
				fmt.Printf("Switched to profile: %s\n", profile)
			}
		}
		return nil
	},
}

func init() {
	addCredentialFlags(createCmd)
	createCmd.Flags().Bool("use", false, "Switch to this profile after creation")
	configCmd.AddCommand(createCmd)
}
