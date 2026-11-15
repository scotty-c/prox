package completion

import (
	"context"
	"fmt"
	"slices"
	"time"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/config"
	"github.com/spf13/cobra"
)

// lookupTimeout bounds the Proxmox calls made while completing. Completion
// runs every time TAB is pressed, so a slow or unreachable cluster must not
// hold up the shell for the full HTTP timeout.
const lookupTimeout = 3 * time.Second

// CompletionFunc is the signature cobra uses for argument completion
type CompletionFunc = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

// commandContext is the context of the command being completed. cobra sets
// it when running a command, and during completion on newer versions; fall
// back to a background context rather than panic if it is missing.
func commandContext(cmd *cobra.Command) context.Context {
	if cmd != nil && cmd.Context() != nil {
		return cmd.Context()
	}
	return context.Background()
}

// clusterResources fetches the cluster listing, giving up after lookupTimeout
func clusterResources(cmd *cobra.Command) ([]c.Resource, error) {
	ctx, cancel := context.WithTimeout(commandContext(cmd), lookupTimeout)
	defer cancel()
	client, err := c.CreateClient()
	if err != nil {
		return nil, err
	}
	return client.GetClusterResources(ctx)
}

// guests completes the VMs or containers (guestType is c.GuestVM or
// c.GuestContainer) whose status is one of statuses, or all of them if none
// are given. Each guest is offered by name and by ID, with a description that
// shells such as zsh and fish show next to the candidate.
func guests(guestType string, statuses ...string) CompletionFunc {
	label := "VM"
	if guestType == c.GuestContainer {
		label = "container"
	}
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		resources, err := clusterResources(cmd)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		var candidates []string
		for _, r := range resources {
			if r.Type != guestType || r.VMID == nil {
				continue
			}
			if len(statuses) > 0 && !slices.Contains(statuses, r.Status) {
				continue
			}
			if r.Name != "" {
				candidates = append(candidates, fmt.Sprintf("%s\t%s %d on %s, %s", r.Name, label, *r.VMID, r.Node, r.Status))
			}
			desc := fmt.Sprintf("%s on %s, %s", label, r.Node, r.Status)
			if r.Name != "" {
				desc = fmt.Sprintf("%s %s on %s, %s", label, r.Name, r.Node, r.Status)
			}
			candidates = append(candidates, fmt.Sprintf("%d\t%s", *r.VMID, desc))
		}
		return candidates, cobra.ShellCompDirectiveNoFileComp
	}
}

// Completion functions for guests. The state-aware ones only offer what the
// command can act on: "prox vm start" has no use for a VM that is running.
var (
	GetVMNames        = guests(c.GuestVM)
	GetContainerNames = guests(c.GuestContainer)

	StoppedVMs        = guests(c.GuestVM, "stopped")
	RunningVMs        = guests(c.GuestVM, "running")
	StoppedContainers = guests(c.GuestContainer, "stopped")
	RunningContainers = guests(c.GuestContainer, "running")
)

// GetNodeNames returns a list of node names for shell completion
func GetNodeNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	resources, err := clusterResources(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	seen := make(map[string]bool)
	var candidates []string
	for _, r := range resources {
		if r.Type == "node" && r.Node != "" && !seen[r.Node] {
			seen[r.Node] = true
			candidates = append(candidates, fmt.Sprintf("%s\t%s", r.Node, r.Status))
		}
	}
	return candidates, cobra.ShellCompDirectiveNoFileComp
}

// GetProfileNames returns a list of configuration profile names for shell completion
func GetProfileNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	profiles, err := config.ListProfiles()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	current := config.GetCurrentProfile()
	candidates := make([]string, 0, len(profiles))
	for _, p := range profiles {
		if p == current {
			p += "\tcurrent profile"
		}
		candidates = append(candidates, p)
	}
	return candidates, cobra.ShellCompDirectiveNoFileComp
}

// FirstArgOnly wraps a completion function so it only completes the first
// positional argument, e.g. "prox vm clone <TAB>" but not "prox vm clone web <TAB>".
func FirstArgOnly(fn CompletionFunc) CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return fn(cmd, args, toComplete)
	}
}

// RegisterNodeFlag adds node name completion to a command's flag
func RegisterNodeFlag(cmd *cobra.Command, flag string) {
	_ = cmd.RegisterFlagCompletionFunc(flag, GetNodeNames)
}
