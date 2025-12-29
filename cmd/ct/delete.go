package cmd

import (
	"fmt"
	"os"

	"github.com/scotty-c/prox/pkg/container"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:     "delete [CONTAINER_ID or NAME] [flags]",
	Aliases: []string{"del", "rm"},
	Short:   "Delete a container",
	Long:    `Delete a container from the Proxmox VE server by providing the container ID or name. The node will be automatically discovered.`,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		nameOrID := args[0]
		force, _ := cmd.Flags().GetBool("force")

		// Confirm deletion unless --force is used
		if !force {
			if !output.Confirm(fmt.Sprintf("Are you sure you want to delete container '%s'? This action cannot be undone", nameOrID)) {
				fmt.Println("Deletion cancelled")
				return
			}
		}

		err := container.DeleteContainer(nameOrID)
		if err != nil {
			output.UserError("deleting container", err)
			os.Exit(1)
		}
	},
}

func init() {
	deleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt (use with caution)")
	ctCmd.AddCommand(deleteCmd)
}
