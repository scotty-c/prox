package completion

import (
	"context"
	"strconv"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/spf13/cobra"
)

// GetVMNames returns a list of VM names for shell completion
func GetVMNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	client, err := c.CreateClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	resources, err := client.GetClusterResources(context.Background())
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var names []string
	for _, resource := range resources {
		if resource.Type == "qemu" {
			// Add VM name
			if resource.Name != "" {
				names = append(names, resource.Name)
			}
			// Also add VM ID as a completion option
			if resource.VMID != nil {
				names = append(names, strconv.Itoa(*resource.VMID))
			}
		}
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

// GetContainerNames returns a list of container names for shell completion
func GetContainerNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	client, err := c.CreateClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	resources, err := client.GetClusterResources(context.Background())
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var names []string
	for _, resource := range resources {
		if resource.Type == "lxc" {
			// Add container name
			if resource.Name != "" {
				names = append(names, resource.Name)
			}
			// Also add container ID as a completion option
			if resource.VMID != nil {
				names = append(names, strconv.Itoa(*resource.VMID))
			}
		}
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

// GetNodeNames returns a list of node names for shell completion
func GetNodeNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	client, err := c.CreateClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	resources, err := client.GetClusterResources(context.Background())
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	nodeSet := make(map[string]bool)
	var names []string

	for _, resource := range resources {
		if resource.Type == "node" && resource.Node != "" && !nodeSet[resource.Node] {
			names = append(names, resource.Node)
			nodeSet[resource.Node] = true
		}
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

// GetProfileNames returns a list of configuration profile names for shell completion
func GetProfileNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// This would require reading from config directory
	// For now, return no completions
	return nil, cobra.ShellCompDirectiveNoFileComp
}
