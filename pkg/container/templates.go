package container

import (
	"context"
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	c "github.com/scotty-c/prox/pkg/client"
)

// ListTemplates lists all available container templates
func ListTemplates(node string) {
	client, err := c.CreateClient()
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return
	}

	fmt.Println("📋 Retrieving container templates...")

	// Get all nodes if no specific node is provided
	nodes := []string{}
	if node != "" {
		nodes = append(nodes, node)
	} else {
		// Get all nodes in the cluster
		clusterNodes, err := getClusterNodes(client)
		if err != nil {
			fmt.Printf("❌ Error getting cluster nodes: %v\n", err)
			return
		}
		nodes = clusterNodes
	}

	var allTemplates []Template
	for _, nodeName := range nodes {
		templates, err := getNodeTemplates(client, nodeName)
		if err != nil {
			fmt.Printf("⚠️  Warning: Could not get templates from node %s: %v\n", nodeName, err)
			continue
		}
		allTemplates = append(allTemplates, templates...)
	}

	if len(allTemplates) == 0 {
		fmt.Println("❌ No container templates found")
		return
	}

	// Display templates in a table
	displayTemplatesTable(allTemplates)
}

// getNodeTemplates gets templates from a specific node
func getNodeTemplates(client *c.ProxmoxClient, node string) ([]Template, error) {
	templates, err := client.GetContainerTemplates(context.Background(), node)
	if err != nil {
		return nil, err
	}

	var result []Template
	for _, template := range templates {
		// Parse template information
		tmpl := Template{
			Name:        template.VolID,
			Description: parseTemplateDescription(template.VolID),
			OS:          parseTemplateOS(template.VolID),
			Version:     parseTemplateVersion(template.VolID),
			Size:        formatSize(template.Size),
			Node:        node,
		}
		result = append(result, tmpl)
	}

	return result, nil
}

// displayTemplatesTable displays templates in a formatted table
func displayTemplatesTable(templates []Template) {
	t := table.NewWriter()
	t.SetTitle("Container Templates")
	t.AppendHeader(table.Row{"OS", "VERSION", "DESCRIPTION", "SIZE", "NODE"})

	for _, template := range templates {
		t.AppendRow(table.Row{
			template.OS,
			template.Version,
			template.Description,
			template.Size,
			template.Node,
		})
	}

	t.SetStyle(table.StyleRounded)
	fmt.Printf("\n%s\n", t.Render())
	fmt.Printf("Found %d container template(s)\n", len(templates))
}

// ResolveTemplate resolves a short template name (e.g., "ubuntu:22.04") to a full template path and node
func ResolveTemplate(shortName string) (*ResolvedTemplate, error) {
	// If the template is already in full format, we need to find which node has it
	if strings.Contains(shortName, ":vztmpl/") {
		client, err := c.CreateClient()
		if err != nil {
			return nil, fmt.Errorf("error creating client: %w", err)
		}

		// Get all templates from all nodes to find which node has this template
		nodes, err := getClusterNodes(client)
		if err != nil {
			return nil, fmt.Errorf("failed to get cluster nodes: %w", err)
		}

		for _, node := range nodes {
			templates, err := getNodeTemplates(client, node)
			if err != nil {
				continue // Skip nodes that are not accessible
			}

			// Check if this node has the template
			for _, template := range templates {
				if template.Name == shortName {
					return &ResolvedTemplate{
						Template: shortName,
						Node:     node,
					}, nil
				}
			}
		}

		return nil, fmt.Errorf("template %s not found on any node", shortName)
	}

	// Check if it's a short format (os:version)
	if !strings.Contains(shortName, ":") {
		return nil, fmt.Errorf("template must be in format 'os:version' (e.g., 'ubuntu:22.04') or full format 'storage:vztmpl/template-name'")
	}

	client, err := c.CreateClient()
	if err != nil {
		return nil, fmt.Errorf("error creating client: %w", err)
	}

	// Get all templates from all nodes
	nodes, err := getClusterNodes(client)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster nodes: %w", err)
	}

	var allTemplates []Template
	for _, node := range nodes {
		templates, err := getNodeTemplates(client, node)
		if err != nil {
			continue // Skip nodes that are not accessible
		}
		allTemplates = append(allTemplates, templates...)
	}

	// Parse the short name
	parts := strings.Split(shortName, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid template format. Use 'os:version' (e.g., 'ubuntu:22.04')")
	}

	requestedOS := strings.ToLower(parts[0])
	requestedVersion := parts[1]

	// Find matching templates
	var matches []Template
	for _, template := range allTemplates {
		templateOS := strings.ToLower(template.OS)
		templateVersion := template.Version

		// Match OS (case-insensitive)
		if templateOS == requestedOS || strings.Contains(templateOS, requestedOS) {
			// Match version (exact or contains)
			if templateVersion == requestedVersion || strings.Contains(templateVersion, requestedVersion) {
				matches = append(matches, template)
			}
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no template found for %s. Use 'prox ct templates' to see available templates", shortName)
	}

	// If multiple matches, prefer the most recent or standard one
	var bestMatch Template
	if len(matches) > 1 {
		// Sort by preference: standard > default > others
		bestMatch = matches[0]
		for _, match := range matches {
			if strings.Contains(strings.ToLower(match.Name), "standard") {
				bestMatch = match
				break
			}
		}
		fmt.Printf("💡 Multiple templates found for %s, using: %s on node %s\n", shortName, bestMatch.Name, bestMatch.Node)
	} else {
		bestMatch = matches[0]
		fmt.Printf("💡 Resolved %s to: %s on node %s\n", shortName, bestMatch.Name, bestMatch.Node)
	}

	return &ResolvedTemplate{
		Template: bestMatch.Name,
		Node:     bestMatch.Node,
	}, nil
}

// ListTemplateShortcuts lists common template shortcuts for user reference
func ListTemplateShortcuts() {
	fmt.Println("🔧 Common template shortcuts:")
	fmt.Println("  ubuntu:22.04    - Ubuntu 22.04 LTS")
	fmt.Println("  ubuntu:20.04    - Ubuntu 20.04 LTS")
	fmt.Println("  debian:12       - Debian 12 (Bookworm)")
	fmt.Println("  debian:11       - Debian 11 (Bullseye)")
	fmt.Println("  alpine:3.18     - Alpine Linux 3.18")
	fmt.Println("  centos:8        - CentOS 8")
	fmt.Println("  fedora:38       - Fedora 38")
	fmt.Println()
	fmt.Println("💡 Use 'prox ct templates' to see all available templates")
}
