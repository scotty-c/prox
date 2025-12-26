package cmd

import (
	"fmt"
	"os"

	"github.com/scotty-c/prox/pkg/container"
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

		err := container.DeleteContainer(nameOrID)
		if err != nil {
			fmt.Printf("Error: Error deleting container: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	ctCmd.AddCommand(deleteCmd)
}
