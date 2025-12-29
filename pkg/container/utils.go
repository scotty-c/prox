package container

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/util"
)

// parseTemplateDescription extracts a readable description from template name
func parseTemplateDescription(volID string) string {
	// Template names are usually in format: local:vztmpl/ubuntu-20.04-standard_20.04-1_amd64.tar.gz
	parts := strings.Split(volID, "/")
	if len(parts) < 2 {
		return volID
	}

	filename := parts[len(parts)-1]
	// Remove .tar.gz extension
	filename = strings.TrimSuffix(filename, ".tar.gz")

	return strings.ReplaceAll(filename, "_", " ")
}

// parseTemplateOS extracts OS name from template
func parseTemplateOS(volID string) string {
	if strings.Contains(volID, "ubuntu") {
		return "Ubuntu"
	} else if strings.Contains(volID, "debian") {
		return "Debian"
	} else if strings.Contains(volID, "centos") {
		return "CentOS"
	} else if strings.Contains(volID, "alpine") {
		return "Alpine"
	} else if strings.Contains(volID, "fedora") {
		return "Fedora"
	}
	return "Unknown"
}

// parseTemplateVersion extracts version from template name
func parseTemplateVersion(volID string) string {
	// Try to extract version numbers like 20.04, 22.04, etc.
	parts := strings.Split(volID, "-")
	for _, part := range parts {
		if strings.Contains(part, ".") && len(part) <= 10 {
			// Likely a version number
			return part
		}
	}
	return "Unknown"
}

// getClusterNodes gets all nodes in the cluster
func getClusterNodes(client c.ProxmoxClientInterface) ([]string, error) {
	resources, err := client.GetClusterResources(context.Background())
	if err != nil {
		return nil, err
	}

	var nodes []string
	nodeSet := make(map[string]bool)

	for _, resource := range resources {
		if resource.Type == "node" && resource.Node != "" && !nodeSet[resource.Node] {
			nodes = append(nodes, resource.Node)
			nodeSet[resource.Node] = true
		}
	}

	return nodes, nil
}

// autoDetectNode automatically detects the best node for container creation
func autoDetectNode(client *c.ProxmoxClient) (string, error) {
	resources, err := client.GetClusterResources(context.Background())
	if err != nil {
		return "", err
	}

	// Find the first available node
	for _, resource := range resources {
		if resource.Type == "node" && resource.Status == "online" {
			return resource.Node, nil
		}
	}

	return "", fmt.Errorf("no online nodes found")
}

// waitForTask waits for a Proxmox task to complete
func waitForTask(ctx context.Context, client c.ProxmoxClientInterface, node, taskID string) error {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " Processing..."
	s.Start()
	defer s.Stop()

	// Exponential backoff configuration
	backoff := 500 * time.Millisecond // Start at 500ms
	maxBackoff := 5 * time.Second     // Cap at 5s

	for {
		task, err := client.GetTaskStatus(ctx, node, taskID)
		if err != nil {
			return fmt.Errorf("failed to get task status: %w", err)
		}

		if task.Status == "stopped" {
			if task.ExitCode == "OK" {
				return nil
			}
			return fmt.Errorf("task failed with exit code: %s", task.ExitCode)
		}

		// Wait with exponential backoff before checking again
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			// Double the backoff for next iteration, cap at maxBackoff
			backoff = backoff * 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// FindByNameOrID finds a container by name or ID
func FindByNameOrID(ctx context.Context, client c.ProxmoxClientInterface, nameOrID string) (*Container, error) {
	// Get cluster resources
	resources, err := client.GetClusterResources(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster resources: %w", err)
	}

	// Try to parse as ID first
	if vmid, err := strconv.Atoi(nameOrID); err == nil {
		// Search by ID
		for _, resource := range resources {
			if resource.Type == "lxc" && resource.VMID != nil && *resource.VMID == vmid {
				return &Container{
					ID:     int(*resource.VMID),
					Name:   resource.Name,
					Status: resource.Status,
					Node:   resource.Node,
				}, nil
			}
		}
	}

	// Search by name and collect all container names for suggestions
	var allContainerNames []string
	for _, resource := range resources {
		if resource.Type == "lxc" {
			if resource.Name == nameOrID {
				return &Container{
					ID:     int(*resource.VMID),
					Name:   resource.Name,
					Status: resource.Status,
					Node:   resource.Node,
				}, nil
			}
			// Collect container names for fuzzy matching
			if resource.Name != "" {
				allContainerNames = append(allContainerNames, resource.Name)
			}
		}
	}

	// Container not found - provide helpful suggestions
	errorMsg := fmt.Sprintf("container '%s' not found", nameOrID)

	// Find similar names using fuzzy matching
	suggestions := util.FindSimilarStrings(nameOrID, allContainerNames, 3)
	if len(suggestions) > 0 {
		errorMsg += "\n\nDid you mean one of these?"
		for _, suggestion := range suggestions {
			errorMsg += fmt.Sprintf("\n  • %s", suggestion)
		}
		errorMsg += "\n\nRun 'prox ct list' to see all available containers"
	} else if len(allContainerNames) > 0 {
		errorMsg += "\n\nRun 'prox ct list' to see all available containers"
	}

	return nil, fmt.Errorf(errorMsg)
}

// ValidateSSHKeys validates SSH public keys format and returns the number of valid keys
func ValidateSSHKeys(sshKeys string) (int, error) {
	if sshKeys == "" {
		return 0, nil
	}

	lines := strings.Split(strings.TrimSpace(sshKeys), "\n")
	validKeys := 0

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue // Skip empty lines
		}

		// Basic SSH key validation - should start with ssh-rsa, ssh-ed25519, ssh-dss, or ecdsa-sha2
		validPrefixes := []string{"ssh-rsa", "ssh-ed25519", "ssh-dss", "ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521"}
		isValid := false
		for _, prefix := range validPrefixes {
			if strings.HasPrefix(line, prefix+" ") {
				isValid = true
				break
			}
		}

		if !isValid {
			return validKeys, fmt.Errorf("line %d: invalid SSH key format (should start with ssh-rsa, ssh-ed25519, etc.)", i+1)
		}

		// Check if key has at least 3 parts (type, key, optional comment)
		parts := strings.Fields(line)
		if len(parts) < 2 {
			return validKeys, fmt.Errorf("line %d: SSH key appears incomplete (missing key data)", i+1)
		}

		validKeys++
	}

	return validKeys, nil
}
