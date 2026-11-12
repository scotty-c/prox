package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/container"
	"github.com/scotty-c/prox/pkg/sshconfig"
	"github.com/scotty-c/prox/pkg/vm"
	"github.com/spf13/cobra"
)

var sshCmd = &cobra.Command{
	Use:     "ssh [VM_OR_CONTAINER_NAME_OR_ID]",
	GroupID: "utilities",
	Short:   "Setup SSH configuration for a VM or container",
	Long: `Lookup the IP address of a VM or container and setup SSH configuration.

This command will:
1. Find the VM or container by name or ID
2. Retrieve its IP address from Proxmox
3. Add or update an entry in your ~/.ssh/config file

Examples:
  prox ssh myvm                       # Setup SSH for VM named 'myvm'
  prox ssh 123                        # Setup SSH for VM/container with ID 123
  prox ssh web-server                 # Setup SSH for VM/container named 'web-server'
  prox ssh mycontainer --user root    # Setup SSH with specific username
  prox ssh myvm --delete              # Delete SSH config entry for 'myvm'
  prox ssh --list                     # List SSH config host entries`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		listFlag, _ := cmd.Flags().GetBool("list")
		deleteFlag, _ := cmd.Flags().GetBool("delete")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		// List mode (no args required; ignore others except dry-run which is harmless here)
		if listFlag {
			if deleteFlag {
				return fmt.Errorf("--list and --delete cannot be used together")
			}
			return listSSHConfigEntries()
		}

		// For add/update or delete we need exactly 1 arg
		if len(args) != 1 {
			return fmt.Errorf("expected 1 argument (resource name/ID or host alias); use --list to list entries")
		}

		nameOrID := args[0]

		if deleteFlag {
			return deleteSSHConfigEntry(nameOrID, dryRun)
		}

		// Get other flags
		username, _ := cmd.Flags().GetString("user")
		port, _ := cmd.Flags().GetInt("port")
		keyPath, _ := cmd.Flags().GetString("key")
		skipHostKeyCheck, _ := cmd.Flags().GetBool("no-host-key-check")

		return setupSSHConfig(nameOrID, username, port, keyPath, dryRun, skipHostKeyCheck)
	},
}

func init() {
	sshCmd.Flags().StringP("user", "u", "", "SSH username (if not specified, uses the resource name)")
	sshCmd.Flags().IntP("port", "p", 22, "SSH port")
	sshCmd.Flags().StringP("key", "k", "", "Path to SSH private key (optional)")
	sshCmd.Flags().Bool("no-host-key-check", false, "Disable SSH host key checking for this entry (for disposable VMs whose keys change on rebuild)")
	sshCmd.Flags().Bool("dry-run", false, "Show what would be added to SSH config without actually modifying it")
	sshCmd.Flags().Bool("delete", false, "Delete SSH config entry for the given VM or container")
	sshCmd.Flags().Bool("list", false, "List SSH config host entries")

	RootCmd.AddCommand(sshCmd)
}

func setupSSHConfig(nameOrID, username string, port int, keyPath string, dryRun bool, skipHostKeyCheck bool) error {
	ctx := context.Background()

	// Validate input
	if nameOrID == "" {
		return fmt.Errorf("VM or container name/ID cannot be empty")
	}

	if port <= 0 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	// Create client
	client, err := client.CreateClient()
	if err != nil {
		return fmt.Errorf("failed to create Proxmox client: %w", err)
	}

	// Try to find as VM first, then as container
	var ip, resourceName, resourceType, node string
	var resourceID int

	vm, vmErr := vm.FindByNameOrID(ctx, client, nameOrID)
	if vmErr == nil {
		// Found as VM
		resourceType = "VM"
		resourceName = vm.Name
		resourceID = vm.ID
		node = vm.Node

		fmt.Printf("Found %s: %s (ID: %d) on node %s\n", resourceType, resourceName, resourceID, node)

		// Get VM IP
		ip, err = client.GetVMIP(ctx, node, resourceID)
		if err != nil {
			return fmt.Errorf("failed to get VM IP: %w", err)
		}
	} else {
		// Try as container
		container, ctErr := container.FindByNameOrID(ctx, client, nameOrID)
		if ctErr != nil {
			return fmt.Errorf("resource '%s' not found as VM or container. VM error: %v, Container error: %v", nameOrID, vmErr, ctErr)
		}

		resourceType = "Container"
		resourceName = container.Name
		resourceID = container.ID
		node = container.Node

		fmt.Printf("Found %s: %s (ID: %d) on node %s\n", resourceType, resourceName, resourceID, node)

		// Get Container IP
		ip, err = client.GetContainerIP(ctx, node, resourceID)
		if err != nil {
			return fmt.Errorf("failed to get container IP: %w", err)
		}
	}

	if ip == "" || ip == "N/A" {
		return fmt.Errorf("no IP address found for %s '%s' (ID: %d). Make sure the resource is running and has an IP assigned", resourceType, resourceName, resourceID)
	}

	// Clean IP address (remove CIDR notation if present)
	if strings.Contains(ip, "/") {
		ip = strings.Split(ip, "/")[0]
	}

	// Validate IP address
	if !isValidIP(ip) {
		return fmt.Errorf("invalid IP address: %s", ip)
	}

	// Use resource name as username if not specified
	if username == "" {
		username = resourceName
	}

	// Validate SSH key path if provided
	if keyPath != "" {
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			return fmt.Errorf("SSH key file does not exist: %s", keyPath)
		}
	}

	// Generate SSH config entry
	hostAlias := resourceName
	sshConfigEntry := generateSSHConfigEntry(hostAlias, ip, username, port, keyPath, skipHostKeyCheck)

	fmt.Printf("IP Address: %s\n", ip)
	fmt.Printf("SSH Username: %s\n", username)
	fmt.Printf("SSH Port: %d\n", port)
	if keyPath != "" {
		fmt.Printf("SSH Key: %s\n", keyPath)
	}

	if dryRun {
		fmt.Printf("\nSSH config entry that would be added:\n\n%s\n", sshConfigEntry)
		fmt.Printf("Tip: To apply these changes, run the command without --dry-run\n")
		return nil
	}

	// Add to SSH config
	err = addToSSHConfig(hostAlias, sshConfigEntry)
	if err != nil {
		return fmt.Errorf("failed to update SSH config: %w", err)
	}

	fmt.Printf("\nSSH configuration updated successfully!\n")
	fmt.Printf("Tip: You can now connect with: ssh %s\n", hostAlias)

	return nil
}

func isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

func generateSSHConfigEntry(host, hostname, user string, port int, keyPath string, skipHostKeyCheck bool) string {
	var config strings.Builder

	config.WriteString(fmt.Sprintf("Host %s\n", host))
	config.WriteString(fmt.Sprintf("    HostName %s\n", hostname))
	config.WriteString(fmt.Sprintf("    User %s\n", user))

	if port != 22 {
		config.WriteString(fmt.Sprintf("    Port %d\n", port))
	}

	if keyPath != "" {
		config.WriteString(fmt.Sprintf("    IdentityFile %s\n", keyPath))
	}

	if skipHostKeyCheck {
		// Opt-in for disposable VMs whose host keys change on every rebuild
		config.WriteString("    StrictHostKeyChecking no\n")
		config.WriteString("    UserKnownHostsFile /dev/null\n")
	}

	return config.String()
}

func sshConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".ssh", "config"), nil
}

func addToSSHConfig(hostAlias, configEntry string) error {
	configPath, err := sshConfigPath()
	if err != nil {
		return err
	}

	// Create .ssh directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Read existing config if it exists
	existingConfig, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read existing SSH config: %w", err)
	}

	newConfig := sshconfig.AddHost(string(existingConfig), hostAlias, configEntry)
	if err := sshconfig.WriteFile(configPath, []byte(newConfig)); err != nil {
		return fmt.Errorf("failed to write SSH config: %w", err)
	}

	return nil
}

func deleteSSHConfigEntry(nameOrID string, dryRun bool) error {
	ctx := context.Background()

	if nameOrID == "" {
		return fmt.Errorf("VM or container name/ID cannot be empty")
	}

	// Try to resolve to actual resource name to match stored host alias
	alias := nameOrID
	if cl, err := client.CreateClient(); err == nil {
		if vmObj, vmErr := vm.FindByNameOrID(ctx, cl, nameOrID); vmErr == nil {
			alias = vmObj.Name
		} else if ctObj, ctErr := container.FindByNameOrID(ctx, cl, nameOrID); ctErr == nil {
			alias = ctObj.Name
		}
	}

	removed, block, err := removeFromSSHConfig(alias, dryRun)
	if err != nil {
		return err
	}

	if dryRun {
		if removed {
			fmt.Printf("[Dry run] Would remove SSH config entry for host '%s':\n\n%s\n", alias, block)
		} else {
			fmt.Printf("[Dry run] No SSH config entry found for host '%s' (nothing to remove)\n", alias)
		}
		return nil
	}

	if removed {
		fmt.Printf("Removed SSH config entry for host '%s'\n", alias)
	} else {
		fmt.Fprintf(os.Stderr, "WARNING: No SSH config entry found for host '%s'\n", alias)
	}
	return nil
}

// removeFromSSHConfig removes a host block from ~/.ssh/config. Returns (removed, removedBlock, error).
func removeFromSSHConfig(hostAlias string, dryRun bool) (bool, string, error) {
	configPath, err := sshConfigPath()
	if err != nil {
		return false, "", err
	}

	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return false, "", nil // Nothing to remove
	}
	if err != nil {
		return false, "", fmt.Errorf("failed to read SSH config: %w", err)
	}

	newConfig, block, removed := sshconfig.RemoveHost(string(data), hostAlias, false)

	// If dry run, do not write changes; just return info
	if dryRun || !removed {
		return removed, block, nil
	}

	if err := sshconfig.WriteFile(configPath, []byte(newConfig)); err != nil {
		return false, "", fmt.Errorf("failed to write updated SSH config: %w", err)
	}

	return removed, block, nil
}

func listSSHConfigEntries() error {
	configPath, err := sshConfigPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("Note: No SSH config file found (~/.ssh/config)")
		return nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read SSH config: %w", err)
	}

	entries := sshconfig.ParseEntries(string(data))

	if len(entries) == 0 {
		fmt.Println("Note: No host entries found in SSH config")
		return nil
	}

	// Build pretty table similar style to VM list
	t := table.NewWriter()
	t.SetTitle("SSH Config Hosts")
	t.AppendHeader(table.Row{"HOST", "HOSTNAME", "USER", "PORT", "IDENTITY"})

	aliasCount := 0
	for _, e := range entries {
		// Ensure defaults
		if e.Port == "" {
			e.Port = "22"
		}
		if len(e.Hosts) == 0 {
			e.Hosts = []string{"(unnamed)"}
		}
		for i, h := range e.Hosts {
			aliasCount++
			if i == 0 {
				// primary row with details
				identity := e.Identity
				if identity == "" {
					identity = ""
				}
				t.AppendRow(table.Row{h, e.HostName, e.User, e.Port, identity})
			} else {
				// additional alias rows
				t.AppendRow(table.Row{h, "", "", "", ""})
			}
		}
	}

	t.SetStyle(table.StyleRounded)
	fmt.Printf("\n%s\n", t.Render())
	fmt.Printf("Found %d SSH host entry(ies) (%d alias name(s))\n", len(entries), aliasCount)
	return nil
}
