package container

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	c "github.com/scotty-c/prox/pkg/client"
)

// formatSize formats size in bytes to human readable format
func formatSize(sizeBytes uint64) string {
	const unit = 1024
	if sizeBytes < unit {
		return fmt.Sprintf("%d B", sizeBytes)
	}
	div, exp := uint64(unit), 0
	for n := sizeBytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(sizeBytes)/float64(div), "KMGTPE"[exp])
}

// formatUptime formats uptime in seconds to human readable format
func formatUptime(uptimeSeconds int64) string {
	if uptimeSeconds <= 0 {
		return "0s"
	}

	days := uptimeSeconds / 86400
	hours := (uptimeSeconds % 86400) / 3600
	minutes := (uptimeSeconds % 3600) / 60
	seconds := uptimeSeconds % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	} else {
		return fmt.Sprintf("%ds", seconds)
	}
}

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
func getClusterNodes(client *c.ProxmoxClient) ([]string, error) {
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
func waitForTask(client *c.ProxmoxClient, node, taskID string) error {
	for {
		task, err := client.GetTaskStatus(context.Background(), node, taskID)
		if err != nil {
			return fmt.Errorf("failed to get task status: %w", err)
		}

		if task.Status == "stopped" {
			if task.ExitCode == "OK" {
				return nil
			}
			return fmt.Errorf("task failed with exit code: %s", task.ExitCode)
		}

		// Wait a bit before checking again
		time.Sleep(c.TaskPollIntervalSeconds * time.Second)
	}
}

// findContainer finds a container by name or ID
func findContainer(client *c.ProxmoxClient, nameOrID string) (*Container, error) {
	// Get cluster resources
	resources, err := client.GetClusterResources(context.Background())
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

	// Search by name
	for _, resource := range resources {
		if resource.Type == "lxc" && resource.Name == nameOrID {
			return &Container{
				ID:     int(*resource.VMID),
				Name:   resource.Name,
				Status: resource.Status,
				Node:   resource.Node,
			}, nil
		}
	}

	return nil, fmt.Errorf("container '%s' not found", nameOrID)
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
