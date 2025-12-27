package container

import (
	"context"
	"fmt"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/output"
)

// StartContainer starts a container by name or ID
func StartContainer(nameOrID string) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}

	output.Info("Starting container %s...\n", nameOrID)

	// Find the container
	container, err := FindByNameOrID(client, nameOrID)
	if err != nil {
		return fmt.Errorf("failed to find container: %w", err)
	}

	// Check if container is already running
	if container.Status == "running" {
		output.Info("Container %s is already running\n", nameOrID)
		return nil
	}

	// Start the container
	taskID, err := client.StartContainer(context.Background(), container.Node, container.ID)
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	output.Result("Task started: %s\n", taskID)
	output.Infoln("Waiting for container to start...")

	// Wait for task completion
	err = waitForTask(client, container.Node, taskID)
	if err != nil {
		return fmt.Errorf("container start failed: %w", err)
	}

	output.Result("Container %s started successfully\n", nameOrID)
	return nil
}

// StopContainer stops a container by name or ID
func StopContainer(nameOrID string) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}

	output.Info("Stopping container %s...\n", nameOrID)

	// Find the container
	container, err := FindByNameOrID(client, nameOrID)
	if err != nil {
		return fmt.Errorf("failed to find container: %w", err)
	}

	// Check if container is already stopped
	if container.Status == "stopped" {
		output.Info("Container %s is already stopped\n", nameOrID)
		return nil
	}

	// Stop the container
	taskID, err := client.StopContainer(context.Background(), container.Node, container.ID)
	if err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	output.Result("Task started: %s\n", taskID)
	output.Infoln("Waiting for container to stop...")

	// Wait for task completion
	err = waitForTask(client, container.Node, taskID)
	if err != nil {
		return fmt.Errorf("container stop failed: %w", err)
	}

	output.Result("Container %s stopped successfully\n", nameOrID)
	return nil
}

// DeleteContainer deletes a container by name or ID
func DeleteContainer(nameOrID string) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}

	output.Info("Deleting container %s...\n", nameOrID)
	output.Infoln("WARNING: This action cannot be undone!")

	// Find the container
	container, err := FindByNameOrID(client, nameOrID)
	if err != nil {
		return fmt.Errorf("failed to find container: %w", err)
	}

	// Delete the container
	taskID, err := client.DeleteContainer(context.Background(), container.Node, container.ID)
	if err != nil {
		return fmt.Errorf("failed to delete container: %w", err)
	}

	output.Result("Task started: %s\n", taskID)
	output.Infoln("Waiting for container deletion...")

	// Wait for task completion
	err = waitForTask(client, container.Node, taskID)
	if err != nil {
		return fmt.Errorf("container deletion failed: %w", err)
	}

	output.Result("Container %s deleted successfully\n", nameOrID)
	return nil
}
