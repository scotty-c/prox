package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
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
  prox ssh --list                     # List SSH config host entries
  prox ssh myvm --connect             # Connect now without editing ~/.ssh/config
  prox ssh myvm --print               # Print the ssh command, e.g. for scripts`,
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

		printFlag, _ := cmd.Flags().GetBool("print")
		connectFlag, _ := cmd.Flags().GetBool("connect")
		if printFlag || connectFlag {
			if printFlag && connectFlag {
				return fmt.Errorf("--print and --connect cannot be used together")
			}
			if deleteFlag || dryRun {
				return fmt.Errorf("--print and --connect cannot be combined with --delete or --dry-run")
			}
		}

		if deleteFlag {
			return deleteSSHConfigEntry(nameOrID, dryRun)
		}

		// Get other flags
		username, _ := cmd.Flags().GetString("user")
		port, _ := cmd.Flags().GetInt("port")
		keyPath, _ := cmd.Flags().GetString("key")
		skipHostKeyCheck, _ := cmd.Flags().GetBool("no-host-key-check")

		if printFlag || connectFlag {
			return runSSH(nameOrID, username, port, keyPath, skipHostKeyCheck, connectFlag)
		}
		return setupSSHConfig(nameOrID, username, port, keyPath, dryRun, skipHostKeyCheck)
	},
}

func init() {
	sshCmd.Flags().StringP("user", "u", "", "SSH username (default: your local username)")
	sshCmd.Flags().IntP("port", "p", 22, "SSH port")
	sshCmd.Flags().StringP("key", "k", "", "Path to SSH private key (optional)")
	sshCmd.Flags().Bool("no-host-key-check", false, "Disable SSH host key checking for this entry (for disposable VMs whose keys change on rebuild)")
	sshCmd.Flags().Bool("dry-run", false, "Show what would be added to SSH config without actually modifying it")
	sshCmd.Flags().Bool("delete", false, "Delete SSH config entry for the given VM or container")
	sshCmd.Flags().Bool("list", false, "List SSH config host entries")
	sshCmd.Flags().Bool("connect", false, "Connect now with ssh instead of editing ~/.ssh/config")
	sshCmd.Flags().Bool("print", false, "Print the ssh command instead of editing ~/.ssh/config")

	RootCmd.AddCommand(sshCmd)
}

// sshTarget is a VM or container to connect to
type sshTarget struct {
	Type string // "VM" or "Container"
	Name string
	ID   int
	Node string
	IP   string
}

// resolveSSHTarget finds a VM or container by name or ID and looks up its IP.
// Progress messages go to info.
func resolveSSHTarget(ctx context.Context, nameOrID string, info io.Writer) (*sshTarget, error) {
	if nameOrID == "" {
		return nil, fmt.Errorf("VM or container name/ID cannot be empty")
	}

	cl, err := client.CreateClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Proxmox client: %w", err)
	}

	// Try to find as VM first, then as container
	t := &sshTarget{}
	vmObj, vmErr := vm.FindByNameOrID(ctx, cl, nameOrID)
	if vmErr == nil {
		t.Type, t.Name, t.ID, t.Node = "VM", vmObj.Name, vmObj.ID, vmObj.Node
		_, _ = fmt.Fprintf(info, "Found %s: %s (ID: %d) on node %s\n", t.Type, t.Name, t.ID, t.Node)
		if t.IP, err = cl.GetVMIP(ctx, t.Node, t.ID); err != nil {
			return nil, fmt.Errorf("failed to get VM IP: %w", err)
		}
	} else {
		ctObj, ctErr := container.FindByNameOrID(ctx, cl, nameOrID)
		if ctErr != nil {
			return nil, fmt.Errorf("resource '%s' not found as VM or container. VM error: %v, Container error: %v", nameOrID, vmErr, ctErr)
		}
		t.Type, t.Name, t.ID, t.Node = "Container", ctObj.Name, ctObj.ID, ctObj.Node
		_, _ = fmt.Fprintf(info, "Found %s: %s (ID: %d) on node %s\n", t.Type, t.Name, t.ID, t.Node)
		if t.IP, err = cl.GetContainerIP(ctx, t.Node, t.ID); err != nil {
			return nil, fmt.Errorf("failed to get container IP: %w", err)
		}
	}

	if t.IP == "" || t.IP == "N/A" {
		return nil, fmt.Errorf("no IP address found for %s '%s' (ID: %d). Make sure the resource is running and has an IP assigned", t.Type, t.Name, t.ID)
	}

	// Clean IP address (remove CIDR notation if present)
	t.IP, _, _ = strings.Cut(t.IP, "/")
	if !isValidIP(t.IP) {
		return nil, fmt.Errorf("invalid IP address: %s", t.IP)
	}
	return t, nil
}

// validateSSHOptions checks the port and key file
func validateSSHOptions(port int, keyPath string) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if keyPath != "" {
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			return fmt.Errorf("SSH key file does not exist: %s", keyPath)
		}
	}
	return nil
}

func setupSSHConfig(nameOrID, username string, port int, keyPath string, dryRun bool, skipHostKeyCheck bool) error {
	if err := validateSSHOptions(port, keyPath); err != nil {
		return err
	}

	target, err := resolveSSHTarget(context.Background(), nameOrID, os.Stdout)
	if err != nil {
		return err
	}
	ip := target.IP

	// Default to the local username, as plain ssh does
	if username == "" {
		username = localUsername()
	}

	// Generate SSH config entry
	hostAlias := target.Name
	sshConfigEntry := generateSSHConfigEntry(hostAlias, ip, username, port, keyPath, skipHostKeyCheck)

	fmt.Printf("IP Address: %s\n", ip)
	if username != "" {
		fmt.Printf("SSH Username: %s\n", username)
	}
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

// sshCommandArgs returns the ssh command line for target (including "ssh")
func sshCommandArgs(ip, username string, port int, keyPath string, skipHostKeyCheck bool) []string {
	args := []string{"ssh"}
	if port != 22 {
		args = append(args, "-p", strconv.Itoa(port))
	}
	if keyPath != "" {
		args = append(args, "-i", keyPath)
	}
	if skipHostKeyCheck {
		args = append(args, "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null")
	}
	// ssh accepts bare IPv4 and IPv6 addresses as the destination
	dest := ip
	if username != "" {
		dest = username + "@" + dest
	}
	return append(args, dest)
}

// shellQuote quotes s for a POSIX shell when it contains special characters
func shellQuote(s string) string {
	isSafe := func(r rune) bool {
		return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("@%+=:,./_-~[]", r)
	}
	if s != "" && strings.IndexFunc(s, func(r rune) bool { return !isSafe(r) }) == -1 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

// runSSH prints or runs an ssh command for a VM or container without
// touching ~/.ssh/config
func runSSH(nameOrID, username string, port int, keyPath string, skipHostKeyCheck, connect bool) error {
	if err := validateSSHOptions(port, keyPath); err != nil {
		return err
	}
	// Status messages go to stderr so --print output can be used directly
	target, err := resolveSSHTarget(context.Background(), nameOrID, os.Stderr)
	if err != nil {
		return err
	}
	if username == "" {
		username = localUsername()
	}
	args := sshCommandArgs(target.IP, username, port, keyPath, skipHostKeyCheck)

	if !connect {
		quoted := make([]string, len(args))
		for i, a := range args {
			quoted[i] = shellQuote(a)
		}
		fmt.Println(strings.Join(quoted, " "))
		return nil
	}

	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh client not found in PATH: %w", err)
	}
	c := exec.Command(sshPath, args[1:]...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	// Let ssh handle Ctrl-C itself
	signal.Ignore(os.Interrupt)
	if err := c.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("failed to run ssh: %w", err)
	}
	return nil
}

func isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// localUsername returns the current user's name without any Windows domain
// prefix, or "" if it can't be determined
func localUsername() string {
	name := ""
	if u, err := user.Current(); err == nil {
		name = u.Username
	}
	if name == "" {
		name = os.Getenv("USER")
	}
	if i := strings.LastIndex(name, `\`); i != -1 {
		name = name[i+1:]
	}
	return name
}

func generateSSHConfigEntry(host, hostname, user string, port int, keyPath string, skipHostKeyCheck bool) string {
	var config strings.Builder

	fmt.Fprintf(&config, "Host %s\n", host)
	fmt.Fprintf(&config, "    HostName %s\n", hostname)
	if user != "" {
		fmt.Fprintf(&config, "    User %s\n", user)
	}

	if port != 22 {
		fmt.Fprintf(&config, "    Port %d\n", port)
	}

	if keyPath != "" {
		fmt.Fprintf(&config, "    IdentityFile %s\n", keyPath)
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
