package client

import (
	"context"
	"fmt"
	"time"
)

// Helper functions for common Proxmox operations
// These complement the existing client.go functionality

// GetClusterStatus returns comprehensive cluster status information
func (c *ProxmoxClient) GetClusterStatus(ctx context.Context) (*ClusterStatus, error) {
	nodes, err := c.GetNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	resources, err := c.GetClusterResources(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster resources: %w", err)
	}

	version, err := c.GetVersion(ctx)
	if err != nil {
		// Version is optional, don't fail if we can't get it
		version = nil
	}

	return &ClusterStatus{
		Nodes:     nodes,
		Resources: resources,
		Version:   version,
	}, nil
}

// WaitForTask waits for a Proxmox task to complete and returns the final status
func (c *ProxmoxClient) WaitForTask(ctx context.Context, node, taskID string) (*Task, error) {
	return PollTask(ctx, c, node, taskID)
}

// TaskStatusGetter is the subset of the client needed to poll a task
type TaskStatusGetter interface {
	GetTaskStatus(ctx context.Context, node, upid string) (*Task, error)
}

// PollTask polls a task with exponential backoff (500ms up to 5s) until it is
// no longer running, and returns its final status.
func PollTask(ctx context.Context, client TaskStatusGetter, node, taskID string) (*Task, error) {
	backoff := 500 * time.Millisecond
	const maxBackoff = 5 * time.Second

	for {
		task, err := client.GetTaskStatus(ctx, node, taskID)
		if err != nil {
			return nil, fmt.Errorf("failed to get task status: %w", err)
		}

		if task.Status != "running" {
			return task, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// WaitForTaskSuccess waits for a task to finish and returns an error if it
// did not finish with exit status OK.
func WaitForTaskSuccess(ctx context.Context, client TaskStatusGetter, node, taskID string) error {
	task, err := PollTask(ctx, client, node, taskID)
	if err != nil {
		return err
	}
	if task.ExitCode != "OK" {
		return fmt.Errorf("task failed with exit code: %s", task.ExitCode)
	}
	return nil
}

// IsVMRunning checks if a VM is currently running
func (c *ProxmoxClient) IsVMRunning(ctx context.Context, vmid int) (bool, error) {
	node, err := c.GetVMNode(ctx, vmid)
	if err != nil {
		return false, err
	}

	status, err := c.GetVMStatus(ctx, node, vmid)
	if err != nil {
		return false, err
	}

	if statusStr, ok := status["status"].(string); ok {
		return statusStr == "running", nil
	}

	return false, fmt.Errorf("unable to determine VM status")
}

// IsContainerRunning checks if a container is currently running
func (c *ProxmoxClient) IsContainerRunning(ctx context.Context, vmid int) (bool, error) {
	// Find which node the container is on
	resources, err := c.GetClusterResources(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get cluster resources: %w", err)
	}

	var node string
	for _, resource := range resources {
		if resource.Type == "lxc" && resource.VMID != nil && *resource.VMID == vmid {
			node = resource.Node
			break
		}
	}

	if node == "" {
		return false, fmt.Errorf("container %d not found in cluster", vmid)
	}

	status, err := c.GetContainerStatus(ctx, node, vmid)
	if err != nil {
		return false, err
	}

	if statusStr, ok := status["status"].(string); ok {
		return statusStr == "running", nil
	}

	return false, fmt.Errorf("unable to determine container status")
}
