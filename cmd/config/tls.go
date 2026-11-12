package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	c "github.com/scotty-c/prox/pkg/config"
	"github.com/spf13/cobra"
)

// applyTLSFlags saves --insecure and --ca-cert to profile if either was given.
// It returns true if anything was saved.
func applyTLSFlags(cmd *cobra.Command, profile string) (bool, error) {
	insecureSet := cmd.Flags().Changed("insecure")
	caCert, _ := cmd.Flags().GetString("ca-cert")
	if !insecureSet && caCert == "" {
		return false, nil
	}

	opts, err := c.ReadTLSOptions(profile)
	if err != nil {
		return false, err
	}
	if insecureSet {
		opts.Insecure, _ = cmd.Flags().GetBool("insecure")
	}
	if caCert != "" {
		abs, err := filepath.Abs(caCert)
		if err != nil {
			return false, fmt.Errorf("invalid CA certificate path: %w", err)
		}
		if _, err := os.Stat(abs); err != nil {
			return false, fmt.Errorf("CA certificate file: %w", err)
		}
		opts.CACert = abs
	}
	if err := c.SetTLSOptions(profile, opts); err != nil {
		return false, err
	}
	return true, nil
}

// saveTLSFlagsOrExit saves TLS flags to profile, printing and exiting on failure
func saveTLSFlagsOrExit(cmd *cobra.Command, profile string) {
	saved, err := applyTLSFlags(cmd, profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error saving TLS settings: %v\n", err)
		os.Exit(1)
	}
	if saved {
		fmt.Printf("TLS settings saved to profile '%s'\n", profile)
	}
}
