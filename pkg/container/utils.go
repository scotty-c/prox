package container

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/util"
)

// templateArchiveSuffixes lists the archive extensions used by Proxmox container templates
var templateArchiveSuffixes = []string{".tar.gz", ".tar.zst", ".tar.xz", ".tar.bz2", ".tgz"}

// knownTemplateOSNames maps template filename prefixes to display names.
// The order matters for the legacy substring fallback in parseTemplateOS.
var knownTemplateOSNames = []struct {
	prefix string
	name   string
}{
	{"ubuntu", "Ubuntu"},
	{"debian", "Debian"},
	{"centos", "CentOS"},
	{"alpine", "Alpine"},
	{"fedora", "Fedora"},
}

// templateFilename returns the template file name without the storage prefix
// or archive extension, e.g. "local:vztmpl/debian-12-standard_12.7-1_amd64.tar.zst"
// becomes "debian-12-standard_12.7-1_amd64".
func templateFilename(volID string) string {
	filename := volID
	if idx := strings.LastIndex(filename, "/"); idx != -1 {
		filename = filename[idx+1:]
	}
	for _, suffix := range templateArchiveSuffixes {
		if strings.HasSuffix(filename, suffix) {
			return strings.TrimSuffix(filename, suffix)
		}
	}
	return filename
}

// parseTemplateDescription extracts a readable description from template name
func parseTemplateDescription(volID string) string {
	// Template names are usually in format: local:vztmpl/ubuntu-20.04-standard_20.04-1_amd64.tar.gz
	if !strings.Contains(volID, "/") {
		return volID
	}
	return strings.ReplaceAll(templateFilename(volID), "_", " ")
}

// parseTemplateOS extracts OS name from template
func parseTemplateOS(volID string) string {
	// Standard Proxmox templates are named <os>-<version>-<flavour>_...
	prefix := strings.ToLower(strings.SplitN(templateFilename(volID), "-", 2)[0])
	for _, known := range knownTemplateOSNames {
		if prefix == known.prefix {
			return known.name
		}
	}

	// Legacy behaviour: look for a known OS name anywhere in the volume ID
	for _, known := range knownTemplateOSNames {
		if strings.Contains(volID, known.prefix) {
			return known.name
		}
	}

	// Otherwise use the filename prefix (e.g. "rockylinux", "archlinux")
	if prefix != "" && strings.Contains(templateFilename(volID), "-") {
		return strings.ToUpper(prefix[:1]) + prefix[1:]
	}
	return "Unknown"
}

// parseTemplateVersion extracts version from template name
func parseTemplateVersion(volID string) string {
	// Standard Proxmox templates are named <os>-<version>-<flavour>_...,
	// e.g. debian-12-standard_12.7-1_amd64 or ubuntu-22.04-standard_22.04-1_amd64
	parts := strings.Split(templateFilename(volID), "-")
	if len(parts) >= 2 && parts[1] != "" && parts[1][0] >= '0' && parts[1][0] <= '9' {
		return parts[1]
	}

	// Fallback: look for something that looks like a dotted version number
	for _, part := range strings.Split(volID, "-") {
		if strings.Contains(part, ".") && len(part) <= 10 {
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

// waitForTask waits for a Proxmox task to complete, showing a spinner
func waitForTask(ctx context.Context, client c.ProxmoxClientInterface, node, taskID string) error {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " Processing..."
	s.Start()
	defer s.Stop()

	return c.WaitForTaskSuccess(ctx, client, node, taskID)
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

// ReadSSHKeys reads SSH public keys (one per line) from r until EOF.
// It returns the non-empty lines joined with newlines and the number of those lines.
func ReadSSHKeys(r io.Reader) (string, int, error) {
	var keyLines []string
	scanner := bufio.NewScanner(r)
	// Allow long lines (e.g. large RSA keys with comments)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			keyLines = append(keyLines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", 0, err
	}
	return strings.Join(keyLines, "\n"), len(keyLines), nil
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
