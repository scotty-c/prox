package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/scotty-c/prox/pkg/vm"
	"github.com/spf13/cobra"
)

// cloneCmd represents the clone command

var cloneCmd = &cobra.Command{
	Use:   "clone <source_name|id> [NEW_VM_ID] [NAME] [flags]",
	Short: "Clone a virtual machine",
	Long: `Clone an existing virtual machine to create a new one with a different ID. The source VM can be specified by name or ID, and its node will be automatically discovered if not specified.

By default, Proxmox attempts to create a linked clone (if supported by the storage backend).
Use the --full flag to create a full clone instead, which copies all disk data and is supported by all storage types.

Examples:
  prox vm clone myvm 200 newvm
  prox vm clone 100 200 newvm --full
  prox vm clone web-server 300 web-clone`,
	Args: cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		sourceNameOrID := args[0]

		// Parse new VM ID
		newID, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Printf("Error: Invalid new VM ID '%s'. Must be a number.\n", args[1])
			os.Exit(1)
		}

		// Get flags
		flagName, _ := cmd.Flags().GetString("name")
		full, _ := cmd.Flags().GetBool("full")

		// Optional positional name support; precedence to positional when provided
		posName := ""
		if len(args) >= 3 {
			posName = args[2]
		}
		name := posName
		if name == "" {
			name = flagName
		} else if flagName != "" && name != flagName {
			fmt.Printf("WARNING: --name \"%s\" ignored; using positional name \"%s\"\n", flagName, name)
		}

		if name == "" {
			fmt.Println("Error: NAME (positional) or --name flag is required")
			cmd.Usage()
			os.Exit(1)
		}

		if err := vm.CloneVMByNameOrID(sourceNameOrID, name, newID, full); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	cloneCmd.Flags().StringP("node", "n", "", "Proxmox node name (optional - will be auto-discovered from source VM if not specified)")
	cloneCmd.Flags().StringP("name", "N", "", "Name for the new virtual machine (alternative to positional NAME)")
	cloneCmd.Flags().BoolP("full", "f", false, "Create a full clone instead of linked clone (copies all disk data, required for some storage types like SMB/NFS)")
	vmCmd.AddCommand(cloneCmd)
}
