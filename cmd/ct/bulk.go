package cmd

import (
	"context"
	"fmt"
	"strconv"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/container"
	"github.com/spf13/cobra"
)

// runOnTaggedContainers runs fn (with the container ID as a string) on every
// container with the --tag value, after confirmation
func runOnTaggedContainers(cmd *cobra.Command, verb string, wait bool, fn func(ctx context.Context, id string) error) error {
	tag, _ := cmd.Flags().GetString("tag")
	yes, _ := cmd.Flags().GetBool("yes")

	cl, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	containers, err := container.FindByTag(cmd.Context(), cl, tag)
	if err != nil {
		return err
	}

	targets := make([]cmdutil.BulkTarget, 0, len(containers))
	for _, ct := range containers {
		id := strconv.Itoa(ct.ID)
		targets = append(targets, cmdutil.BulkTarget{
			Label: fmt.Sprintf("%s (ID: %d) on %s", ct.Name, ct.ID, ct.Node),
			Run:   func(ctx context.Context) error { return fn(ctx, id) },
		})
	}
	return cmdutil.RunBulk(cmd.Context(), cl, verb, "container", tag, targets, yes, wait)
}
