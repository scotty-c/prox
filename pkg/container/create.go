package container

import (
	"context"
	"fmt"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/output"
)

// CreateContainer creates a new LXC container
func CreateContainer(node, name, template string, vmid int, memory, disk int, cores int, password, sshKeys, storage string) error {
	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}

	output.Info("Creating container %s...\n", name)

	// Resolve template if it's in short format
	resolvedTemplate, err := ResolveTemplate(template)
	if err != nil {
		return fmt.Errorf("failed to resolve template: %w", err)
	}
	template = resolvedTemplate.Template

	// Use the node where the template is located, unless a specific node is provided
	if node == "" {
		node = resolvedTemplate.Node
		output.Info("Using node: %s (where template is located)\n", node)
	} else {
		// If user specified a node, verify the template exists on that node
		if node != resolvedTemplate.Node {
			output.Error("WARNING: Template is on node %s, but you specified node %s. Using template's node %s\n",
				resolvedTemplate.Node, node, resolvedTemplate.Node)
			node = resolvedTemplate.Node
		}
		output.Info("Using node: %s\n", node)
	}

	// Get next available VMID if not provided
	if vmid == 0 {
		nextID, err := client.GetNextVMID(ctx)
		if err != nil {
			return fmt.Errorf("failed to get next VM ID: %w", err)
		}
		vmid = nextID
		output.Info("Using VM ID: %d\n", vmid)
	}

	// Use default storage if not specified
	if storage == "" {
		storage = "local-lvm"
		output.Info("Using default storage: %s\n", storage)
	} else {
		output.Info("Using storage: %s\n", storage)
	}

	// Prepare container parameters
	params := map[string]interface{}{
		"hostname":     name,
		"ostemplate":   template,
		"memory":       memory,
		"rootfs":       fmt.Sprintf("%s:%d", storage, disk),
		"cores":        cores,
		"net0":         "name=eth0,bridge=vmbr0,ip=dhcp",
		"start":        0, // Don't start automatically
		"unprivileged": 1, // Create as unprivileged container
	}

	// Add password if provided
	if password != "" {
		params["password"] = password
	}

	// Add SSH keys if provided
	if sshKeys != "" {
		validKeys, err := ValidateSSHKeys(sshKeys)
		if err != nil {
			return fmt.Errorf("SSH key validation failed: %w", err)
		}
		if validKeys > 0 {
			params["ssh-public-keys"] = sshKeys
			output.Info("Added %d SSH public key(s)\n", validKeys)
		}
	}

	// Create the container
	taskID, err := client.CreateContainer(ctx, node, vmid, params)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	output.Result("Task started: %s\n", taskID)

	// Wait for task completion
	err = waitForTask(ctx, client, node, taskID)
	if err != nil {
		return fmt.Errorf("container creation failed: %w", err)
	}

	output.Result("Container %s (ID: %d) created successfully on node %s\n", name, vmid, node)
	output.Info("Tip: Use 'prox ct start %s' to start the container\n", name)

	return nil
}
