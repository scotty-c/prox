package cmd

import (
	"fmt"
	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/completion"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/scotty-c/prox/pkg/container"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create <name> <template>",
	Short: "Create a new LXC container",
	Long: `Create a new LXC container with the specified name and template.

The template can be specified in two formats:
1. Short format: os:version (e.g., ubuntu:22.04, debian:12, alpine:3.18)
2. Full format: storage:vztmpl/template-name (e.g., local:vztmpl/ubuntu-22.04-standard_22.04-1_amd64.tar.zst)

The short format will automatically resolve to the full template path.

Storage Configuration:
- Use --storage to specify where the container rootfs will be stored
- Defaults to 'local-lvm', or the first storage that can hold containers if there is no local-lvm
- Common storage options: local-lvm, local-zfs, nfs-storage, ceph

SSH Key Options:
- Use --ssh-keys-file to specify a path to your SSH public key file (e.g., ~/.ssh/id_rsa.pub)
- Use --ssh-keys-file=- to read SSH keys from stdin
- Multiple keys can be included in the file, one per line

Examples:
  prox ct create mycontainer ubuntu:22.04
  prox ct create web debian:12 --cpu 2 --start     # create and start it
  prox ct create webapp debian:12 --memory 2048 --disk 32
  prox ct create dev-env alpine:3.18 --cores 2 --node node1 --ssh-keys-file ~/.ssh/id_rsa.pub
  prox ct create mycontainer ubuntu:22.04 --storage local-zfs --disk 16
  prox ct create web debian:12 --bridge vmbr1 --vlan 20 --ip 10.0.20.5/24 --gateway 10.0.20.1
  prox ct create mycontainer local:vztmpl/ubuntu-22.04-standard_22.04-1_amd64.tar.zst --ssh-keys-file /path/to/keys.pub`,
	Args: func(cmd *cobra.Command, args []string) error {
		// Support either positional <name> <template> OR --name/--template flags.
		// Valid: 2 positional args, or 0 positional with both flags provided.
		if len(args) == 2 {
			return nil
		}
		if len(args) == 0 {
			nameFlag, _ := cmd.Flags().GetString("name")
			tmplFlag, _ := cmd.Flags().GetString("template")
			if strings.TrimSpace(nameFlag) == "" || strings.TrimSpace(tmplFlag) == "" {
				return fmt.Errorf("requires <name> <template> or --name and --template")
			}
			return nil
		}
		return fmt.Errorf("requires <name> <template> (2 args) or use --name and --template flags")
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		wait := cmdutil.ResolveWait(cmd, true)
		// Resolve name/template from positional or flags with precedence to positional.
		posName := ""
		posTemplate := ""
		if len(args) >= 2 {
			posName = args[0]
			posTemplate = args[1]
		}

		nameFlag, _ := cmd.Flags().GetString("name")
		tmplFlag, _ := cmd.Flags().GetString("template")

		name := posName
		if name == "" {
			name = strings.TrimSpace(nameFlag)
		} else if strings.TrimSpace(nameFlag) != "" && name != strings.TrimSpace(nameFlag) {
			fmt.Fprintf(os.Stderr, "WARNING: --name \"%s\" ignored; using positional name \"%s\"\n", strings.TrimSpace(nameFlag), name)
		}

		template := posTemplate
		if template == "" {
			template = strings.TrimSpace(tmplFlag)
		} else if strings.TrimSpace(tmplFlag) != "" && template != strings.TrimSpace(tmplFlag) {
			fmt.Fprintf(os.Stderr, "WARNING: --template \"%s\" ignored; using positional template \"%s\"\n", strings.TrimSpace(tmplFlag), template)
		}

		// Get flags
		node, _ := cmd.Flags().GetString("node")
		vmidStr, _ := cmd.Flags().GetString("vmid")
		memory, _ := cmd.Flags().GetInt("memory")
		disk, _ := cmd.Flags().GetInt("disk")
		cores, _ := cmd.Flags().GetInt("cores")
		password, _ := cmd.Flags().GetString("password")
		sshKeysFile, _ := cmd.Flags().GetString("ssh-keys-file")
		promptPassword, _ := cmd.Flags().GetBool("prompt-password")
		storage, _ := cmd.Flags().GetString("storage")
		bridge, _ := cmd.Flags().GetString("bridge")
		ip, _ := cmd.Flags().GetString("ip")
		gateway, _ := cmd.Flags().GetString("gateway")
		vlan, _ := cmd.Flags().GetInt("vlan")
		start, _ := cmd.Flags().GetBool("start")

		// --cpu and --id are the names 'vm create' uses; they mean the same as
		// --cores and --vmid
		if cmd.Flags().Changed("cpu") {
			cpu, _ := cmd.Flags().GetInt("cpu")
			if cmd.Flags().Changed("cores") && cpu != cores {
				return fmt.Errorf("--cpu %d and --cores %d disagree; they are the same setting, use one", cpu, cores)
			}
			cores = cpu
		}
		if id, _ := cmd.Flags().GetInt("id"); id != 0 {
			if vmidStr != "" && vmidStr != strconv.Itoa(id) {
				return fmt.Errorf("--id %d and --vmid %s disagree; they are the same setting, use one", id, vmidStr)
			}
			vmidStr = strconv.Itoa(id)
		}
		if start && !wait {
			return fmt.Errorf("--start needs the container to be created first; drop --no-wait")
		}

		// Parse VMID if provided
		var vmid int
		if vmidStr != "" {
			var err error
			vmid, err = strconv.Atoi(vmidStr)
			if err != nil {
				return fmt.Errorf("Invalid VMID: %s", vmidStr)
			}
		}

		// Handle password input
		if promptPassword {
			fmt.Print("Enter password for container: ")
			passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return fmt.Errorf("reading password: %w", err)
			}
			password = string(passwordBytes)
			fmt.Println() // Add newline after password input
		}

		// Handle SSH keys
		var sshKeys string
		if sshKeysFile != "" {
			if sshKeysFile == "-" {
				// Read from stdin
				if term.IsTerminal(int(os.Stdin.Fd())) {
					fmt.Println("Enter SSH public keys (press Ctrl+D when finished):")
				}
				keys, lineCount, err := container.ReadSSHKeys(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading SSH keys from stdin: %w", err)
				}
				sshKeys = keys
				if sshKeys != "" {
					fmt.Printf("Read %d lines of SSH keys from stdin\n", lineCount)
				}
			} else {
				// Read from file
				fmt.Printf("Reading SSH keys from: %s\n", sshKeysFile)
				keyBytes, err := os.ReadFile(sshKeysFile)
				if err != nil {
					return fmt.Errorf("reading SSH keys file '%s': %w (make sure the file exists and is readable)", sshKeysFile, err)
				}
				sshKeys = strings.TrimSpace(string(keyBytes))
				if sshKeys == "" {
					fmt.Fprintf(os.Stderr, "WARNING: SSH keys file '%s' is empty\n", sshKeysFile)
				} else {
					keyLines := strings.Split(sshKeys, "\n")
					nonEmptyLines := 0
					for _, line := range keyLines {
						if strings.TrimSpace(line) != "" {
							nonEmptyLines++
						}
					}
					fmt.Printf("Read %d SSH key(s) from file\n", nonEmptyLines)
				}
			}

			// Validate SSH keys if provided
			if sshKeys != "" {
				validKeys, err := container.ValidateSSHKeys(sshKeys)
				if err != nil {
					return fmt.Errorf("SSH key validation failed: %w (keys should look like ssh-ed25519 AAAA... or ssh-rsa AAAA...)", err)
				}
				if validKeys == 0 {
					fmt.Fprintln(os.Stderr, "WARNING: No valid SSH keys found")
				} else {
					fmt.Printf("Validated %d SSH key(s)\n", validKeys)
				}
			}
		}

		// Create the container
		err := container.CreateContainerWithOptions(container.CreateOptions{
			Node: node, Name: name, Template: template, VMID: vmid,
			Memory: memory, Disk: disk, Cores: cores,
			Password: password, SSHKeys: sshKeys, Storage: storage,
			Network: container.NetworkOptions{Bridge: bridge, IP: ip, Gateway: gateway, VLAN: vlan},
			Wait:    wait,
			Start:   start,
		})
		if err != nil {
			return fmt.Errorf("creating container: %w", err)
		}
		return nil
	},
}

func init() {
	cmdutil.AddWaitFlags(createCmd, true)
	ctCmd.AddCommand(createCmd)

	// Add flags
	createCmd.Flags().StringP("name", "N", "", "Container name (alternative to positional <name>)")
	createCmd.Flags().StringP("template", "t", "", "Container template (alternative to positional <template>)")
	createCmd.Flags().StringP("node", "n", "", "Proxmox node to create container on (uses template's node if not specified)")
	createCmd.Flags().String("vmid", "", "Container ID (auto-generated if not specified)")
	createCmd.Flags().IntP("memory", "m", 512, "Memory in MB")
	createCmd.Flags().IntP("disk", "d", 8, "Disk size in GB")
	createCmd.Flags().IntP("cores", "c", 1, "Number of CPU cores")
	createCmd.Flags().Int("cpu", 0, "Number of CPU cores (same as --cores; the name 'vm create' uses)")
	createCmd.Flags().Int("id", 0, "Container ID (same as --vmid; the name 'vm create' uses)")
	createCmd.Flags().Bool("start", false, "Start the container after creating it")
	createCmd.Flags().StringP("password", "p", "", "Root password for container")
	createCmd.Flags().Bool("prompt-password", false, "Prompt for password interactively")
	createCmd.Flags().String("ssh-keys-file", "", "Path to SSH public key file (e.g., ~/.ssh/id_rsa.pub) or '-' to read from stdin")
	createCmd.Flags().StringP("storage", "s", "", "Storage location for container rootfs (defaults to local-lvm, or the first container storage if there is no local-lvm)")
	createCmd.Flags().String("bridge", "vmbr0", "Network bridge for eth0")
	createCmd.Flags().String("ip", "dhcp", "IPv4 address for eth0: dhcp, or an address with prefix length (e.g. 10.0.0.5/24)")
	createCmd.Flags().String("gateway", "", "IPv4 gateway for eth0 (with a static --ip)")
	createCmd.Flags().Int("vlan", 0, "VLAN tag for eth0 (0 = untagged)")
	completion.RegisterNodeFlag(createCmd, "node")
}
