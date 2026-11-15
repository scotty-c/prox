package snapshot

import (
	"context"
	"fmt"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/completion"
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
	}
	cmdutil.MakeGroup(root)

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
			return snapshotNames(cmd.Context(), guestType, args[0], node(cmd))
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	// wrap adds what was being done to an error, and passes nil through
	wrap := func(action string, err error) error {
		if err != nil {
			return fmt.Errorf("%s: %w", action, err)
		}
		return nil
	}

	list := &cobra.Command{
		Use:               "list <name|id>",
		Aliases:           []string{"ls"},
		Short:             fmt.Sprintf("List a %s's snapshots", label),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.FirstArgOnly(names),
		RunE: func(cmd *cobra.Command, args []string) error {
			return wrap("listing snapshots", List(cmd.Context(), guestType, args[0], node(cmd)))
		},
	}
	addNode(list)

	create := &cobra.Command{
		Use:               "create <name|id> <snapshot>",
		Short:             fmt.Sprintf("Take a snapshot of a %s", label),
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completion.FirstArgOnly(names),
		RunE: func(cmd *cobra.Command, args []string) error {
			description, _ := cmd.Flags().GetString("description")
			vmState := false
			if f := cmd.Flags().Lookup("vmstate"); f != nil {
				vmState, _ = cmd.Flags().GetBool("vmstate")
			}
			wait := cmdutil.ResolveWait(cmd, true)
			return wrap("creating snapshot", Create(cmd.Context(), guestType, args[0], node(cmd), args[1], description, vmState, wait))
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
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.SkipConfirm(cmd) {
				_, t, err := prepare(cmd.Context(), guestType, args[0], node(cmd))
				if err != nil {
					return wrap("rolling back", err)
				}
				ok, err := cmdutil.Confirm(
					fmt.Sprintf("Roll %s '%s' (ID %d, on %s) back to snapshot '%s'? Changes since the snapshot will be lost", label, t.name, t.id, t.node, args[1]),
					"--yes", "Rollback cancelled")
				if err != nil || !ok {
					return err
				}
			}
			wait := cmdutil.ResolveWait(cmd, true)
			return wrap("rolling back", Rollback(cmd.Context(), guestType, args[0], node(cmd), args[1], wait))
		},
	}
	rollback.Flags().BoolP("force", "f", false, "Skip the confirmation prompt (same as --yes)")
	cmdutil.AddYesFlag(rollback)
	addNode(rollback)
	cmdutil.AddWaitFlags(rollback, true)

	del := &cobra.Command{
		Use:               "delete <name|id> <snapshot>",
		Aliases:           []string{"rm", "del"},
		Short:             "Delete a snapshot",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: complete,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.SkipConfirm(cmd) {
				_, t, err := prepare(cmd.Context(), guestType, args[0], node(cmd))
				if err != nil {
					return wrap("deleting snapshot", err)
				}
				ok, err := cmdutil.Confirm(
					fmt.Sprintf("Delete snapshot '%s' of %s '%s' (ID %d, on %s)?", args[1], label, t.name, t.id, t.node),
					"--yes", "Deletion cancelled")
				if err != nil || !ok {
					return err
				}
			}
			wait := cmdutil.ResolveWait(cmd, true)
			return wrap("deleting snapshot", Delete(cmd.Context(), guestType, args[0], node(cmd), args[1], wait))
		},
	}
	del.Flags().BoolP("force", "f", false, "Skip the confirmation prompt (same as --yes)")
	cmdutil.AddYesFlag(del)
	addNode(del)
	cmdutil.AddWaitFlags(del, true)

	root.AddCommand(list, create, rollback, del)
	return root
}

// snapshotNames completes snapshot names for a VM or container
func snapshotNames(ctx context.Context, guestType, nameOrID, node string) ([]string, cobra.ShellCompDirective) {
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
