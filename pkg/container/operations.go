package container

import (
	"context"
	"fmt"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/output"
)

// StartContainer starts a container by name or ID
func StartContainer(nameOrID string) error {
	ctx := context.Background()
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

	output.Result("Task started: %s\n", taskID)

	// Wait for task completion
	err = waitForTask(ctx, client, container.Node, taskID)
	if err != nil {
		return fmt.Errorf("container start failed: %w", err)
	}

	output.Result("Container %s started successfully\n", nameOrID)
	return nil
}

// StopContainer stops a container by name or ID
func StopContainer(nameOrID string) error {
	return StopContainerWithOptions(nameOrID, false)
}

// StopContainerWithOptions stops a container by name or ID. With force, the
// container is stopped immediately instead of being shut down cleanly.
func StopContainerWithOptions(nameOrID string, force bool) error {
	ctx := context.Background()
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

	output.Result("Task started: %s\n", taskID)

	// Wait for task completion
	err = waitForTask(ctx, client, container.Node, taskID)
	if err != nil {
		return fmt.Errorf("container stop failed: %w", err)
	}

	output.Result("Container %s stopped successfully\n", nameOrID)
	return nil
}

// DeleteContainer deletes a container by name or ID
func DeleteContainer(nameOrID string) error {
	ctx := context.Background()
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

	output.Result("Task started: %s\n", taskID)

	// Wait for task completion
	err = waitForTask(ctx, client, container.Node, taskID)
	if err != nil {
		return fmt.Errorf("container deletion failed: %w", err)
	}

	output.Result("Container %s deleted successfully\n", nameOrID)
	return nil
}
