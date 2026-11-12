package snapshot

import (
	"context"
	"fmt"
	"os"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

// NewCommand builds the "snapshot" command tree for VMs (c.GuestVM) or
// containers (c.GuestContainer)
func NewCommand(guestType string) *cobra.Command {
	label, parent, names := "VM", "vm", completion.GetVMNames
	if guestType == c.GuestContainer {
		label, parent, names = "container", "ct", completion.GetContainerNames
	}

	root := &cobra.Command{
		Use:     "snapshot",
		Aliases: []string{"snap"},
		Short:   fmt.Sprintf("Manage %s snapshots", label),
		Long: fmt.Sprintf(`List, create, roll back and delete snapshots of a %s.

Examples:
  prox %[2]s snapshot list web
  prox %[2]s snapshot create web before-upgrade --description "pre apt upgrade"
  prox %[2]s snapshot rollback web before-upgrade
  prox %[2]s snapshot delete web before-upgrade`, label, parent),
		Run: func(cmd *cobra.Command, args []string) { _ = cmd.Help() },
	}

	// node returns --node for VMs; containers are always looked up cluster-wide
	node := func(cmd *cobra.Command) string {
		if f := cmd.Flags().Lookup("node"); f != nil {
			return f.Value.String()
		}
		return ""
	}
	addNode := func(cmd *cobra.Command) {
		if guestType == c.GuestVM {
			cmd.Flags().StringP("node", "n", "", "Only match VMs on this node")
			completion.RegisterNodeFlag(cmd, "node")
		}
	}
	// Complete the VM/container first, then its snapshot names
	complete := func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return names(cmd, args, toComplete)
		case 1:
			return snapshotNames(guestType, args[0], node(cmd))
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	exitOn := func(action string, err error) {
		if err != nil {
			output.UserError(action, err)
			os.Exit(1)
		}
	}

	list := &cobra.Command{
		Use:               "list <name|id>",
		Aliases:           []string{"ls"},
		Short:             fmt.Sprintf("List a %s's snapshots", label),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.FirstArgOnly(names),
		Run: func(cmd *cobra.Command, args []string) {
			exitOn("listing snapshots", List(guestType, args[0], node(cmd)))
		},
	}
	addNode(list)

	create := &cobra.Command{
		Use:               "create <name|id> <snapshot>",
		Short:             fmt.Sprintf("Take a snapshot of a %s", label),
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completion.FirstArgOnly(names),
		Run: func(cmd *cobra.Command, args []string) {
			description, _ := cmd.Flags().GetString("description")
			vmState := false
			if f := cmd.Flags().Lookup("vmstate"); f != nil {
				vmState, _ = cmd.Flags().GetBool("vmstate")
			}
			wait := cmdutil.ResolveWait(cmd, true)
			exitOn("creating snapshot", Create(guestType, args[0], node(cmd), args[1], description, vmState, wait))
		},
	}
	create.Flags().StringP("description", "d", "", "Snapshot description")
	if guestType == c.GuestVM {
		create.Flags().Bool("vmstate", false, "Also save the VM's memory, so a rollback resumes it running")
	}
	addNode(create)
	cmdutil.AddWaitFlags(create, true)

	rollback := &cobra.Command{
		Use:               "rollback <name|id> <snapshot>",
		Short:             fmt.Sprintf("Roll a %s back to a snapshot", label),
		Long:              fmt.Sprintf("Roll a %s back to a snapshot. Changes made since the snapshot are lost.", label),
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: complete,
		Run: func(cmd *cobra.Command, args []string) {
			force, _ := cmd.Flags().GetBool("force")
			if !force && !output.Confirm(fmt.Sprintf("Roll %s '%s' back to snapshot '%s'? Changes since the snapshot will be lost", label, args[0], args[1])) {
				fmt.Println("Rollback cancelled")
				return
			}
			wait := cmdutil.ResolveWait(cmd, true)
			exitOn("rolling back", Rollback(guestType, args[0], node(cmd), args[1], wait))
		},
	}
	rollback.Flags().BoolP("force", "f", false, "Skip the confirmation prompt")
	addNode(rollback)
	cmdutil.AddWaitFlags(rollback, true)

	del := &cobra.Command{
		Use:               "delete <name|id> <snapshot>",
		Aliases:           []string{"rm", "del"},
		Short:             "Delete a snapshot",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: complete,
		Run: func(cmd *cobra.Command, args []string) {
			force, _ := cmd.Flags().GetBool("force")
			if !force && !output.Confirm(fmt.Sprintf("Delete snapshot '%s' of %s '%s'?", args[1], label, args[0])) {
				fmt.Println("Deletion cancelled")
				return
			}
			wait := cmdutil.ResolveWait(cmd, true)
			exitOn("deleting snapshot", Delete(guestType, args[0], node(cmd), args[1], wait))
		},
	}
	del.Flags().BoolP("force", "f", false, "Skip the confirmation prompt")
	addNode(del)
	cmdutil.AddWaitFlags(del, true)

	root.AddCommand(list, create, rollback, del)
	return root
}

// snapshotNames completes snapshot names for a VM or container
func snapshotNames(guestType, nameOrID, node string) ([]string, cobra.ShellCompDirective) {
	ctx := context.Background()
	cl, err := c.CreateClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	t, err := resolve(ctx, cl, guestType, nameOrID, node)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	snapshots, err := cl.ListSnapshots(ctx, t.node, t.guestType, t.id)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	names := make([]string, 0, len(snapshots))
	for _, s := range snapshots {
		names = append(names, s.Name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
