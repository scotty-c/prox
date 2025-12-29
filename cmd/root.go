/*
Copyright © 2024 prox contributors
*/
package cmd

import (
	"os"

	"github.com/scotty-c/prox/cmd/node"
	c "github.com/scotty-c/prox/pkg/config"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "prox",
	Short: "Modern CLI for Proxmox VE 8+ virtualization management",
	Long: `prox is a modern command-line interface for managing Proxmox VE 8+ environments.
It provides secure, efficient tools for virtual machine and container management.

Features:
  • Secure encrypted configuration storage with AES-256
  • Automatic node discovery for VM operations
  • Modern API client compatible with Proxmox VE 8+
  • Comprehensive VM lifecycle management
  • LXC container template management
  • Enhanced error handling and user feedback
  • Environment variable support for CI/CD scenarios

Configuration:
  prox supports multiple configuration profiles for different environments.
  Profiles are stored in ~/.prox/profiles/ directory.

  Environment variables for CI/CD and automation:
    PROX_URL      - Proxmox server URL (e.g., https://proxmox.example.com:8006)
    PROX_USER     - Username (e.g., root@pam)
    PROX_PASSWORD - Password or API token

Examples:
  prox config setup                    # Configure connection to Proxmox server
  prox config create homelab -u user -p pass -l https://homelab:8006  # Create profile
  prox config list                     # List all profiles
  prox config use homelab              # Switch to homelab profile
  prox --profile production vm list   # Use specific profile for one command
  prox vm list                        # List all virtual machines
  prox vm start myvm                  # Start a virtual machine
  prox vm clone source-vm new-vm      # Clone a virtual machine
  prox vm describe myvm               # Show detailed VM information
  prox ct templates                   # List available container templates
  prox ct create mycontainer ubuntu:22.04    # Create container with short template format
  prox ct start mycontainer           # Start a container
  prox ct stop mycontainer            # Stop a container
  prox ct describe mycontainer        # Show detailed container information
  prox ct shortcuts                   # Show common template shortcuts
  prox container templates -n node1   # List templates from specific node
  prox ssh myvm                       # Setup SSH config for VM/container access
  prox logs <upid>                    # View task logs

CI/CD Usage:
  export PROX_URL=https://proxmox.example.com:8006
  export PROX_USER=automation@pve
  export PROX_PASSWORD=your-token-here
  prox vm list                        # Works without config file

Use "prox [command] --help" for more information about a command.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Set quiet mode based on flag
		quiet, _ := cmd.Flags().GetBool("quiet")
		output.SetQuiet(quiet)

		// Set profile override if flag is provided
		profile, _ := cmd.Flags().GetString("profile")
		if profile != "" {
			c.SetProfileOverride(profile)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Add persistent flags available to all commands
	RootCmd.PersistentFlags().BoolP("quiet", "q", false, "Suppress non-essential output (for scripting)")
	RootCmd.PersistentFlags().String("profile", "", "Use a specific configuration profile")

	// Define command groups for better organization
	RootCmd.AddGroup(
		&cobra.Group{
			ID:    "core",
			Title: "Core Commands:",
		},
		&cobra.Group{
			ID:    "management",
			Title: "Resource Management:",
		},
		&cobra.Group{
			ID:    "utilities",
			Title: "Utility Commands:",
		},
	)

	// Register node command group
	RootCmd.AddCommand(node.NodeCmd)
	// ...existing code...
}
