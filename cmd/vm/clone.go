package cmd

import (
	"fmt"
	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/completion"
	"os"

	"github.com/scotty-c/prox/pkg/vm"
	"github.com/spf13/cobra"
)

// cloneCmd represents the clone command

var cloneCmd = &cobra.Command{
	Use:   "clone <source_name|id> [new_id] [name] [flags]",
	Short: "Clone a virtual machine",
	Long: `Clone an existing virtual machine to create a new one. Without a new ID the next free one is used. The source VM can be specified by name or ID, and its node will be automatically discovered if not specified.

By default, Proxmox attempts to create a linked clone (if supported by the storage backend).
Use the --full flag to create a full clone instead, which copies all disk data and is supported by all storage types.

Examples:
  prox vm clone myvm newvm              # next free ID
  prox vm clone myvm 200 newvm
  prox vm clone 100 200 newvm --full
  prox vm clone web-server 300 web-clone`,
	ValidArgsFunction: completion.FirstArgOnly(completion.GetVMNames),
	Args:              cobra.RangeArgs(1, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceNameOrID := args[0]

		newID, posName, err := cmdutil.ParseCloneArgs(args[1:])
		if err != nil {
			return err
		}

		// Get flags
		flagName, _ := cmd.Flags().GetString("name")
		full, _ := cmd.Flags().GetBool("full")
		wait := cmdutil.ResolveWait(cmd, true)
		node, _ := cmd.Flags().GetString("node")

		// Positional name wins over --name
		name := posName
		if name == "" {
			name = flagName
		} else if flagName != "" && name != flagName {
			fmt.Fprintf(os.Stderr, "WARNING: --name \"%s\" ignored; using positional name \"%s\"\n", flagName, name)
		}

		if name == "" {
			return fmt.Errorf("a name for the new VM is required: give NAME as an argument or use --name")
		}

		if err := vm.CloneVMByNameOrIDWithWait(cmd.Context(), sourceNameOrID, node, name, newID, full, wait); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	cmdutil.AddWaitFlags(cloneCmd, true)
	cloneCmd.Flags().StringP("node", "n", "", "Only match source VMs on this node (optional - auto-discovered if not specified)")
	cloneCmd.Flags().StringP("name", "N", "", "Name for the new virtual machine (alternative to positional NAME)")
	cloneCmd.Flags().BoolP("full", "f", false, "Create a full clone instead of linked clone (copies all disk data, required for some storage types like SMB/NFS)")
	vmCmd.AddCommand(cloneCmd)
	completion.RegisterNodeFlag(cloneCmd, "node")
}
