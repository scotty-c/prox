package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/cmdutil"

	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/container"
	"github.com/scotty-c/prox/pkg/output"
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
	Run: func(cmd *cobra.Command, args []string) {
		wait := cmdutil.ResolveWait(cmd, true)
		nameOrID := args[0]
		force, _ := cmd.Flags().GetBool("force")

		cl, err := c.CreateClient()
		if err != nil {
			output.ClientError(err)
			os.Exit(1)
		}
		// Look the container up first so the question says exactly what will be deleted
		target, err := container.FindByNameOrID(context.Background(), cl, nameOrID)
		if err != nil {
			output.ContainerError("delete", err)
			os.Exit(1)
		}

		if !force && !cmdutil.ConfirmOrExit(
			fmt.Sprintf("Delete container '%s' (ID %d, %s, on %s)? This action cannot be undone", target.Name, target.ID, target.Status, target.Node),
			"--force", "Deletion cancelled") {
			return
		}

		// Delete by ID: the name may be shared by another container
		if err := container.DeleteContainerWithWait(strconv.Itoa(target.ID), wait); err != nil {
			output.UserError("deleting container", err)
			os.Exit(1)
		}
	},
}

func init() {
	cmdutil.AddWaitFlags(deleteCmd, true)
	deleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt (use with caution)")
	ctCmd.AddCommand(deleteCmd)
}
