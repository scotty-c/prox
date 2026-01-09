package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/scotty-c/prox/pkg/container"
	"github.com/scotty-c/prox/pkg/output"
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
- Defaults to 'local-lvm' if not specified
- Common storage options: local-lvm, local-zfs, nfs-storage, ceph

SSH Key Options:
- Use --ssh-keys-file to specify a path to your SSH public key file (e.g., ~/.ssh/id_rsa.pub)
- Use --ssh-keys-file=- to read SSH keys from stdin
- Multiple keys can be included in the file, one per line

Examples:
  prox ct create mycontainer ubuntu:22.04
  prox ct create webapp debian:12 --memory 2048 --disk 32
  prox ct create dev-env alpine:3.18 --cores 2 --node node1 --ssh-keys-file ~/.ssh/id_rsa.pub
  prox ct create mycontainer ubuntu:22.04 --storage local-zfs --disk 16
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
	Run: func(cmd *cobra.Command, args []string) {
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
			fmt.Printf("WARNING: --name \"%s\" ignored; using positional name \"%s\"\n", strings.TrimSpace(nameFlag), name)
		}

		template := posTemplate
		if template == "" {
			template = strings.TrimSpace(tmplFlag)
		} else if strings.TrimSpace(tmplFlag) != "" && template != strings.TrimSpace(tmplFlag) {
			fmt.Printf("WARNING: --template \"%s\" ignored; using positional template \"%s\"\n", strings.TrimSpace(tmplFlag), template)
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

		// Parse VMID if provided
		var vmid int
		if vmidStr != "" {
			var err error
			vmid, err = strconv.Atoi(vmidStr)
			if err != nil {
				fmt.Printf("Error: Invalid VMID: %s\n", vmidStr)
				os.Exit(1)
			}
		}

		// Handle password input
		if promptPassword {
			fmt.Print("Enter password for container: ")
			passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				output.UserError("reading password", err)
				os.Exit(1)
			}
			password = string(passwordBytes)
			fmt.Println() // Add newline after password input
		}

		// Handle SSH keys
		var sshKeys string
		if sshKeysFile != "" {
			if sshKeysFile == "-" {
				// Read from stdin
				fmt.Println("Enter SSH public keys (press Ctrl+D when finished):")
				var keyLines []string
				for {
					var line string
					_, err := fmt.Scanln(&line)
					if err != nil {
						break
					}
					keyLines = append(keyLines, line)
				}
				sshKeys = strings.Join(keyLines, "\n")
				if sshKeys != "" {
					fmt.Printf("Read %d lines of SSH keys from stdin\n", len(keyLines))
				}
			} else {
				// Read from file
				fmt.Printf("Reading SSH keys from: %s\n", sshKeysFile)
				keyBytes, err := os.ReadFile(sshKeysFile)
				if err != nil {
					output.UserError(fmt.Sprintf("reading SSH keys file '%s'", sshKeysFile), err)
					fmt.Println("Tip: Make sure the file exists and is readable")
					os.Exit(1)
				}
				sshKeys = strings.TrimSpace(string(keyBytes))
				if sshKeys == "" {
					fmt.Printf("WARNING: SSH keys file '%s' is empty\n", sshKeysFile)
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
					fmt.Printf("Error: SSH key validation failed: %v\n", err)
					fmt.Println("Tip: Make sure your SSH keys are in the correct format (ssh-rsa, ssh-ed25519, etc.)")
					os.Exit(1)
				}
				if validKeys == 0 {
					fmt.Println("WARNING: No valid SSH keys found")
				} else {
					fmt.Printf("Validated %d SSH key(s)\n", validKeys)
				}
			}
		}

		// Create the container
		err := container.CreateContainer(node, name, template, vmid, memory, disk, cores, password, sshKeys, storage)
		if err != nil {
			output.UserError("creating container", err)
			os.Exit(1)
		}
	},
}

func init() {
	ctCmd.AddCommand(createCmd)

	// Add flags
	createCmd.Flags().StringP("name", "N", "", "Container name (alternative to positional <name>)")
	createCmd.Flags().StringP("template", "t", "", "Container template (alternative to positional <template>)")
	createCmd.Flags().StringP("node", "n", "", "Proxmox node to create container on (uses template's node if not specified)")
	createCmd.Flags().String("vmid", "", "Container ID (auto-generated if not specified)")
	createCmd.Flags().IntP("memory", "m", 512, "Memory in MB")
	createCmd.Flags().IntP("disk", "d", 8, "Disk size in GB")
	createCmd.Flags().IntP("cores", "c", 1, "Number of CPU cores")
	createCmd.Flags().StringP("password", "p", "", "Root password for container")
	createCmd.Flags().Bool("prompt-password", false, "Prompt for password interactively")
	createCmd.Flags().String("ssh-keys-file", "", "Path to SSH public key file (e.g., ~/.ssh/id_rsa.pub) or '-' to read from stdin")
	createCmd.Flags().StringP("storage", "s", "", "Storage location for container rootfs (defaults to local-lvm)")
}
