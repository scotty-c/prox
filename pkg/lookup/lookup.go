// Package lookup finds VMs and containers in the cluster by name or ID. VMs
// and containers are the same kind of thing to Proxmox (a "guest" with an ID
// and a node), so one implementation serves both; Kind says how to word the
// messages.
package lookup

import (
	"context"
	"fmt"
	"strconv"

	"github.com/scotty-c/prox/pkg/apierr"
	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/util"
)

// Kind describes one kind of guest
type Kind struct {
	Type          string // cluster resource type: "qemu" or "lxc"
	Label         string // singular noun for messages: "VM" or "container"
	Plural        string // for "see all available ...": "virtual machines" or "containers"
	ListCommand   string // "prox vm list"
	AmbiguityHint string // what to do when a name matches several guests
}

// VM and Container are the kinds of guest Proxmox runs
var (
	VM = Kind{
		Type: c.GuestVM, Label: "VM", Plural: "virtual machines", ListCommand: "prox vm list",
		AmbiguityHint: "Use the VM ID instead, or --node to pick the node",
	}
	Container = Kind{
		Type: c.GuestContainer, Label: "container", Plural: "containers", ListCommand: "prox ct list",
		AmbiguityHint: "Use the container ID instead",
	}
)

// Find returns the guest of the given kind with this name or ID. If node is
// not empty only guests on that node are considered. A name shared by several
// guests is an error (apierr.ErrAmbiguous) rather than a guess, and a name
// that matches none is apierr.ErrNotFound with suggestions in the message.
func Find(ctx context.Context, client c.ProxmoxClientInterface, kind Kind, nameOrID, node string) (c.Resource, error) {
	resources, err := client.GetClusterResources(ctx)
	if err != nil {
		return c.Resource{}, fmt.Errorf("failed to get cluster resources: %w", err)
	}

	candidates := make([]c.Resource, 0, len(resources))
	for _, r := range resources {
		if r.Type == kind.Type && r.VMID != nil && (node == "" || r.Node == node) {
			candidates = append(candidates, r)
		}
	}

	// An ID wins over a name that happens to look like a number
	if id, convErr := strconv.Atoi(nameOrID); convErr == nil {
		for _, r := range candidates {
			if *r.VMID == id {
				return r, nil
			}
		}
	}

	var matches []c.Resource
	var names []string
	for _, r := range candidates {
		if r.Name == nameOrID {
			matches = append(matches, r)
		}
		if r.Name != "" {
			names = append(names, r.Name)
		}
	}

	switch {
	case len(matches) == 1:
		return matches[0], nil
	case len(matches) > 1:
		// Proxmox allows duplicate names; don't guess which one was meant
		msg := fmt.Sprintf("more than one %s is named '%s':", kind.Label, nameOrID)
		for _, m := range matches {
			msg += fmt.Sprintf("\n  • ID %d on node %s", *m.VMID, m.Node)
		}
		msg += "\n\n" + kind.AmbiguityHint
		return c.Resource{}, apierr.WithKind(apierr.ErrAmbiguous, msg)
	}
	return c.Resource{}, apierr.WithKind(apierr.ErrNotFound, notFoundMessage(kind, nameOrID, node, names))
}

func notFoundMessage(kind Kind, nameOrID, node string, names []string) string {
	msg := fmt.Sprintf("%s '%s' not found", kind.Label, nameOrID)
	if node != "" {
		msg += fmt.Sprintf(" on node '%s'", node)
	}
	seeAll := fmt.Sprintf("\n\nRun '%s' to see all available %s", kind.ListCommand, kind.Plural)
	if suggestions := util.FindSimilarStrings(nameOrID, names, 3); len(suggestions) > 0 {
		msg += "\n\nDid you mean one of these?"
		for _, s := range suggestions {
			msg += "\n  • " + s
		}
		msg += seeAll
	} else if len(names) > 0 {
		msg += seeAll
	}
	return msg
}

// CheckIDFree returns an error if a VM or container already uses id (they
// share one ID space)
func CheckIDFree(ctx context.Context, client c.ProxmoxClientInterface, id int) error {
	resources, err := client.GetClusterResources(ctx)
	if err != nil {
		return fmt.Errorf("failed to get cluster resources: %w", err)
	}
	for _, r := range resources {
		if (r.Type == c.GuestVM || r.Type == c.GuestContainer) && r.VMID != nil && *r.VMID == id {
			kind := VM.Label
			if r.Type == c.GuestContainer {
				kind = Container.Label
			}
			return fmt.Errorf("ID %d is already in use by %s '%s' on node %s", id, kind, r.Name, r.Node)
		}
	}
	return nil
}
