package cmd

import (
	"fmt"
	"os"

	"github.com/scotty-c/prox/pkg/vm"
	"github.com/spf13/cobra"
)

// migrateCmd represents the migrate command for VMs
var migrateCmd = &cobra.Command{
	Use:   "migrate <VM_ID> <target_node> [flags]",
	Short: "Migrate a virtual machine to another node",
	Long: `Migrate a virtual machine from its current node to another node in the Proxmox cluster.

The migration can be performed online (VM continues running) or offline (VM is stopped during migration).
The source node will be automatically discovered if not specified.

Examples:
  prox vm migrate 100 node2                    # Offline migration to node2
  prox vm migrate 100 node2 --online           # Online migration (VM stays running)
  prox vm migrate 100 node2 --with-local-disks # Include local disks in migration
  prox vm migrate 100 node2 --source node1     # Specify source node explicitly`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		// Get VM ID and target node from positional arguments
		vmID := args[0]
		targetNode := args[1]

		// Convert VM ID string to int
		id := 0
		if _, err := fmt.Sscanf(vmID, "%d", &id); err != nil {
			fmt.Printf("❌ Invalid VM ID '%s'. Must be a number.\n", vmID)
			os.Exit(1)
		}

		// Get flags
		sourceNode, _ := cmd.Flags().GetString("source")
		online, _ := cmd.Flags().GetBool("online")
		withLocalDisks, _ := cmd.Flags().GetBool("with-local-disks")

		// Validate target node is not empty
		if targetNode == "" {
			fmt.Println("❌ Target node cannot be empty")
			os.Exit(1)
		}

		// Perform the migration
		vm.MigrateVm(id, sourceNode, targetNode, online, withLocalDisks)
	},
}

func init() {
	migrateCmd.Flags().StringP("source", "s", "", "Source node name (auto-discovered if not specified)")
	migrateCmd.Flags().Bool("online", false, "Perform online migration (VM continues running)")
	migrateCmd.Flags().Bool("with-local-disks", false, "Include local disks in migration")
	vmCmd.AddCommand(migrateCmd)
}
