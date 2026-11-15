package container

import (
	"context"
	"fmt"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/output"
)

// StartContainerWithWait starts a container, optionally without waiting
func StartContainerWithWait(ctx context.Context, nameOrID string, wait bool) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}

	output.Info("Starting container %s...\n", nameOrID)

	// Find the container
	container, err := FindByNameOrID(ctx, client, nameOrID)
	if err != nil {
		return fmt.Errorf("failed to find container: %w", err)
	}

	// Check if container is already running
	if container.Status == "running" {
		output.Info("Container %s is already running\n", nameOrID)
		return nil
	}

	// Start the container
	taskID, err := client.StartContainer(ctx, container.Node, container.ID)
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	if err := cmdutil.FinishTask(ctx, client, container.Node, taskID, wait, fmt.Sprintf("Container %s started", nameOrID)); err != nil {
		return fmt.Errorf("container start failed: %w", err)
	}
	return nil
}

// StopContainerWithOptions stops a container by name or ID. With force, the
// container is stopped immediately instead of being shut down cleanly.
// Without wait it returns as soon as the task has started.
func StopContainerWithOptions(ctx context.Context, nameOrID string, force bool, wait bool) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}

	output.Info("Stopping container %s...\n", nameOrID)

	// Find the container
	container, err := FindByNameOrID(ctx, client, nameOrID)
	if err != nil {
		return fmt.Errorf("failed to find container: %w", err)
	}

	// Check if container is already stopped
	if container.Status == "stopped" {
		output.Info("Container %s is already stopped\n", nameOrID)
		return nil
	}

	// Stop the container
	stop := client.StopContainer
	if force {
		output.Info("Force stopping container %s...\n", nameOrID)
		stop = client.ForceStopContainer
	}
	taskID, err := stop(ctx, container.Node, container.ID)
	if err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	if err := cmdutil.FinishTask(ctx, client, container.Node, taskID, wait, fmt.Sprintf("Container %s stopped", nameOrID)); err != nil {
		return fmt.Errorf("container stop failed: %w", err)
	}
	return nil
}

// DeleteContainerWithWait deletes a container, optionally without waiting
func DeleteContainerWithWait(ctx context.Context, nameOrID string, wait bool) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}

	output.Info("Deleting container %s...\n", nameOrID)
	output.Infoln("WARNING: This action cannot be undone!")

	// Find the container
	container, err := FindByNameOrID(ctx, client, nameOrID)
	if err != nil {
		return fmt.Errorf("failed to find container: %w", err)
	}

	// Delete the container
	taskID, err := client.DeleteContainer(ctx, container.Node, container.ID)
	if err != nil {
		return fmt.Errorf("failed to delete container: %w", err)
	}

	if err := cmdutil.FinishTask(ctx, client, container.Node, taskID, wait, fmt.Sprintf("Container %s deleted", nameOrID)); err != nil {
		return fmt.Errorf("container deletion failed: %w", err)
	}
	return nil
}
