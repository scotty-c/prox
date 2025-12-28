package container

import (
	"context"
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/output"
)

// ListTemplates lists all available container templates
func ListTemplates(node string) {
	client, err := c.CreateClient()
	if err != nil {
		output.Error("Error creating client: %v\n", err)
		return
	}

	output.Infoln("Retrieving container templates...")

	// Get all nodes if no specific node is provided
	nodes := []string{}
	if node != "" {
		nodes = append(nodes, node)
	} else {
		// Get all nodes in the cluster
		clusterNodes, err := getClusterNodes(client)
		if err != nil {
			output.Error("Error: Error getting cluster nodes: %v\n", err)
			return
		}
		nodes = clusterNodes
	}

	var allTemplates []Template
	for _, nodeName := range nodes {
		templates, err := getNodeTemplates(client, nodeName)
		if err != nil {
			output.Error("WARNING: Warning: Could not get templates from node %s: %v\n", nodeName, err)
			continue
		}
		allTemplates = append(allTemplates, templates...)
	}

	if len(allTemplates) == 0 {
		output.Errorln("Error: No container templates found")
		return
	}

	// Display templates in a table
	displayTemplatesTable(allTemplates)
}

// getNodeTemplates gets templates from a specific node
func getNodeTemplates(client c.ProxmoxClientInterface, node string) ([]Template, error) {
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
	output.Result("\n%s\n", t.Render())
	output.Result("Found %d container template(s)\n", len(templates))
}

// ResolveTemplate resolves a short template name (e.g., "ubuntu:22.04") to a full template path and node
func ResolveTemplate(shortName string) (*ResolvedTemplate, error) {
	// Check if it's a short format (os:version) without full path
	if !strings.Contains(shortName, ":") {
		return nil, fmt.Errorf("template must be in format 'os:version' (e.g., 'ubuntu:22.04') or full format 'storage:vztmpl/template-name'")
	}

	// Create client once
	client, err := c.CreateClient()
	if err != nil {
		return nil, fmt.Errorf("error creating client: %w", err)
	}

	// Get all nodes once
	nodes, err := getClusterNodes(client)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster nodes: %w", err)
	}

	// Get all templates from all nodes once
	var allTemplates []Template
	for _, node := range nodes {
		templates, err := getNodeTemplates(client, node)
		if err != nil {
			continue // Skip nodes that are not accessible
		}
		allTemplates = append(allTemplates, templates...)
	}

	// If the template is already in full format, find which node has it
	if strings.Contains(shortName, ":vztmpl/") {
		for _, template := range allTemplates {
			if template.Name == shortName {
				return &ResolvedTemplate{
					Template: shortName,
					Node:     template.Node,
				}, nil
			}
		}
		return nil, fmt.Errorf("template %s not found on any node", shortName)
	}

	// Parse the short name
	parts := strings.Split(shortName, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid template format. Use 'os:version' (e.g., 'ubuntu:22.04')")
	}

	requestedOS := strings.ToLower(parts[0])
	requestedVersion := parts[1]

	// Build index map for O(1) exact lookups
	indexMap := make(map[string][]Template)
	for _, template := range allTemplates {
		key := strings.ToLower(template.OS) + ":" + template.Version
		indexMap[key] = append(indexMap[key], template)
	}

	// Try exact match first (O(1))
	var matches []Template
	exactKey := requestedOS + ":" + requestedVersion
	if exactMatches, found := indexMap[exactKey]; found {
		matches = exactMatches
	} else {
		// Fall back to flexible matching with contains (O(n))
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
		output.Info("Tip: Multiple templates found for %s, using: %s on node %s\n", shortName, bestMatch.Name, bestMatch.Node)
	} else {
		bestMatch = matches[0]
		output.Info("Tip: Resolved %s to: %s on node %s\n", shortName, bestMatch.Name, bestMatch.Node)
	}

	return &ResolvedTemplate{
		Template: bestMatch.Name,
		Node:     bestMatch.Node,
	}, nil
}

// ListTemplateShortcuts lists common template shortcuts for user reference
func ListTemplateShortcuts() {
	output.Resultln("🔧 Common template shortcuts:")
	output.Resultln("  ubuntu:22.04    - Ubuntu 22.04 LTS")
	output.Resultln("  ubuntu:20.04    - Ubuntu 20.04 LTS")
	output.Resultln("  debian:12       - Debian 12 (Bookworm)")
	output.Resultln("  debian:11       - Debian 11 (Bullseye)")
	output.Resultln("  alpine:3.18     - Alpine Linux 3.18")
	output.Resultln("  centos:8        - CentOS 8")
	output.Resultln("  fedora:38       - Fedora 38")
	output.Resultln("")
	output.Info("Tip: Use 'prox ct templates' to see all available templates\n")
}
