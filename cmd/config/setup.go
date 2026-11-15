package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	c "github.com/scotty-c/prox/pkg/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// setupCmd represents the setup command

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Save connection details for the current profile",
	Long: `Save connection details for the current profile.

Certificates are verified by default. If your Proxmox server uses its default
self-signed certificate, add --ca-cert with a copy of /etc/pve/pve-root-ca.pem
from a node, or --insecure to skip verification.

Example:
  prox config setup -u root@pam -p secret -l https://pve.example.com:8006 --ca-cert ~/pve-root-ca.pem`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if c.Check() {
			fmt.Printf("Config already exists for profile '%s'. Use 'prox config update' to change it\n", c.GetCurrentProfile())
			if err := saveTLSFlags(cmd, c.GetCurrentProfile()); err != nil {
				return err
			}
			return nil
		}

		username, password, url, err := readCredentialFlags(cmd)
		if err != nil {
			return err
		}

		// On a terminal, ask for anything not given as a flag rather than
		// saving an incomplete profile. Scripts are unaffected.
		if term.IsTerminal(int(os.Stdin.Fd())) {
			if username, password, url, err = promptMissing(username, password, url); err != nil {
				return fmt.Errorf("reading input: %w", err)
			}
			if url, _, err = c.NormalizeURL(url); err != nil {
				return err
			}
		}

		method, err := c.Create(username, password, url)
		if err != nil {
			return fmt.Errorf("setting up config: %w", err)
		}
		if err := saveTLSFlags(cmd, c.GetCurrentProfile()); err != nil {
			return err
		}
		fmt.Printf("Config setup completed successfully (credentials stored %s)\n", c.DescribeMethod(method))
		return nil
	},
}

// promptMissing asks for any connection details that are empty
func promptMissing(username, password, url string) (string, string, string, error) {
	reader := bufio.NewReader(os.Stdin)
	ask := func(label, current string) (string, error) {
		if current != "" {
			return current, nil
		}
		fmt.Printf("%s: ", label)
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			return "", err
		}
		return strings.TrimSpace(line), nil
	}

	var err error
	if url, err = ask("Proxmox URL (e.g. https://pve.example.com:8006)", url); err != nil {
		return "", "", "", err
	}
	if username, err = ask("Username (e.g. root@pam, or token ID user@realm!name)", username); err != nil {
		return "", "", "", err
	}
	if password == "" {
		fmt.Print("Password (or token secret): ")
		bytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", "", "", err
		}
		password = string(bytes)
	}
	return username, password, url, nil
}

func init() {
	addCredentialFlags(setupCmd)
	configCmd.AddCommand(setupCmd)
}
