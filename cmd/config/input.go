package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	c "github.com/scotty-c/prox/pkg/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// addCredentialFlags registers the connection flags that setup, create and
// update share
func addCredentialFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("username", "u", "", "Username for the Proxmox VE API (or an API token ID, user@realm!name)")
	cmd.Flags().StringP("password", "p", "", "Password (or API token secret); prefer --password-stdin, which keeps it out of shell history and process listings")
	cmd.Flags().Bool("password-stdin", false, "Read the password (or token secret) from stdin instead of --password")
	cmd.Flags().StringP("url", "l", "", "URL of the Proxmox VE API, e.g. https://pve.example.com:8006")
}

// readCredentialFlags reads the flags added by addCredentialFlags. With
// --password-stdin the password comes from stdin, and a URL, if given, is
// tidied with config.NormalizeURL (printing a note to stderr if it looks
// incomplete). Values that were not given are returned empty.
func readCredentialFlags(cmd *cobra.Command) (username, password, url string, err error) {
	username, _ = cmd.Flags().GetString("username")
	password, _ = cmd.Flags().GetString("password")
	url, _ = cmd.Flags().GetString("url")

	if fromStdin, _ := cmd.Flags().GetBool("password-stdin"); fromStdin {
		if password != "" {
			return "", "", "", errors.New("use either --password or --password-stdin, not both")
		}
		if password, err = readPasswordFromStdin(); err != nil {
			return "", "", "", err
		}
	}

	if url != "" {
		normalized, note, err := c.NormalizeURL(url)
		if err != nil {
			return "", "", "", err
		}
		if note != "" {
			fmt.Fprintln(os.Stderr, "Note:", note)
		}
		url = normalized
	}
	return username, password, url, nil
}

// readPasswordFromStdin reads one line from stdin. On a terminal it asks for
// the password without echoing it.
func readPasswordFromStdin() (string, error) {
	var password string
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(os.Stderr, "Password (or token secret): ")
		bytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", fmt.Errorf("reading password: %w", err)
		}
		password = string(bytes)
	} else {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return "", errors.New("no password received on stdin")
		}
		password = strings.TrimRight(line, "\r\n")
	}
	if password == "" {
		return "", errors.New("the password read from stdin is empty")
	}
	return password, nil
}
