package cmd

import (
	"fmt"
	"strconv"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/cmdutil"

	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/container"
	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:               "delete [CONTAINER_ID or NAME] [flags]",
	Aliases:           []string{"del", "rm"},
	Short:             "Delete a container",
	Long:              `Delete a container from the Proxmox VE server by providing the container ID or name. The node will be automatically discovered.`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completion.FirstArgOnly(completion.GetContainerNames),
	RunE: func(cmd *cobra.Command, args []string) error {
		wait := cmdutil.ResolveWait(cmd, true)
		nameOrID := args[0]

		cl, err := c.CreateClient()
		if err != nil {
			return err
		}
		// Look the container up first so the question says exactly what will be deleted
		target, err := container.FindByNameOrID(cmd.Context(), cl, nameOrID)
		if err != nil {
			return err
		}

		if !cmdutil.SkipConfirm(cmd) {
			ok, err := cmdutil.Confirm(
				fmt.Sprintf("Delete container '%s' (ID %d, %s, on %s)? This action cannot be undone", target.Name, target.ID, target.Status, target.Node),
				"--yes", "Deletion cancelled")
			if err != nil || !ok {
				return err
			}
		}

		// Delete by ID: the name may be shared by another container
		if err := container.DeleteContainerWithWait(cmd.Context(), strconv.Itoa(target.ID), wait); err != nil {
			return fmt.Errorf("deleting container: %w", err)
		}
		return nil
	},
}

func init() {
	cmdutil.AddWaitFlags(deleteCmd, true)
	deleteCmd.Flags().BoolP("force", "f", false, "Skip the confirmation prompt (same as --yes)")
	cmdutil.AddYesFlag(deleteCmd)
	ctCmd.AddCommand(deleteCmd)
}
