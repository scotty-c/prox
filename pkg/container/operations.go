package container

import (
	"context"
	"fmt"

	c "github.com/scotty-c/prox/pkg/client"
)

// StartContainer starts a container by name or ID
func StartContainer(nameOrID string) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}

	fmt.Printf("🚀 Starting container %s...\n", nameOrID)

	// Find the container
	container, err := findContainer(client, nameOrID)
	if err != nil {
		return fmt.Errorf("failed to find container: %w", err)
	}

	// Check if container is already running
	if container.Status == "running" {
		fmt.Printf("✅ Container %s is already running\n", nameOrID)
		return nil
	}

	// Start the container
	taskID, err := client.StartContainer(context.Background(), container.Node, container.ID)
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	fmt.Printf("⏳ Task started: %s\n", taskID)
	fmt.Println("🔄 Waiting for container to start...")

	// Wait for task completion
	err = waitForTask(client, container.Node, taskID)
	if err != nil {
		return fmt.Errorf("container start failed: %w", err)
	}

	fmt.Printf("✅ Container %s started successfully\n", nameOrID)
	return nil
}

// StopContainer stops a container by name or ID
func StopContainer(nameOrID string) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}

	fmt.Printf("🛑 Stopping container %s...\n", nameOrID)

	// Find the container
	container, err := findContainer(client, nameOrID)
	if err != nil {
		return fmt.Errorf("failed to find container: %w", err)
	}

	// Check if container is already stopped
	if container.Status == "stopped" {
		fmt.Printf("✅ Container %s is already stopped\n", nameOrID)
		return nil
	}

	// Stop the container
	taskID, err := client.StopContainer(context.Background(), container.Node, container.ID)
	if err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	fmt.Printf("⏳ Task started: %s\n", taskID)
	fmt.Println("🔄 Waiting for container to stop...")

	// Wait for task completion
	err = waitForTask(client, container.Node, taskID)
	if err != nil {
		return fmt.Errorf("container stop failed: %w", err)
	}

	fmt.Printf("✅ Container %s stopped successfully\n", nameOrID)
	return nil
}

// DeleteContainer deletes a container by name or ID
func DeleteContainer(nameOrID string) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}

	fmt.Printf("🗑️  Deleting container %s...\n", nameOrID)
	fmt.Println("⚠️  This action cannot be undone!")

	// Find the container
	container, err := findContainer(client, nameOrID)
	if err != nil {
		return fmt.Errorf("failed to find container: %w", err)
	}

	// Delete the container
	taskID, err := client.DeleteContainer(context.Background(), container.Node, container.ID)
	if err != nil {
		return fmt.Errorf("failed to delete container: %w", err)
	}

	fmt.Printf("⏳ Task started: %s\n", taskID)
	fmt.Println("🔄 Waiting for container deletion...")

	// Wait for task completion
	err = waitForTask(client, container.Node, taskID)
	if err != nil {
		return fmt.Errorf("container deletion failed: %w", err)
	}

	fmt.Printf("✅ Container %s deleted successfully\n", nameOrID)
	return nil
}
