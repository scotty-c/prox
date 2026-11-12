package cmd

import (
	"context"
	"fmt"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/vm"
	"github.com/spf13/cobra"
)

// runOnTaggedVMs runs fn on every VM with the --tag value, after confirmation
func runOnTaggedVMs(cmd *cobra.Command, verb string, fn func(ctx context.Context, cl c.ProxmoxClientInterface, v vm.VM) error) error {
	tag, _ := cmd.Flags().GetString("tag")
	node, _ := cmd.Flags().GetString("node")
	yes, _ := cmd.Flags().GetBool("yes")

	ctx := context.Background()
	cl, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	vms, err := vm.FindByTag(ctx, cl, tag, node)
	if err != nil {
		return err
	}

	targets := make([]cmdutil.BulkTarget, 0, len(vms))
	for _, v := range vms {
		v := v
		targets = append(targets, cmdutil.BulkTarget{
			Label: fmt.Sprintf("%s (ID: %d) on %s", v.Name, v.ID, v.Node),
			Run:   func() error { return fn(ctx, cl, v) },
		})
	}
	return cmdutil.RunBulk(verb, "VM", tag, targets, yes)
}
