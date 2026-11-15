package container

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/output"
)

// CreateOptions describes a container to create
type CreateOptions struct {
	Node     string // empty: the node holding the template
	Name     string
	Template string
	VMID     int // 0: next free ID
	Memory   int // MB
	Disk     int // GB
	Cores    int
	Password string
	SSHKeys  string
	Storage  string // empty: local-lvm if available, otherwise the first rootdir storage
	Network  NetworkOptions
	Wait     bool
	Start    bool // start it once created (needs Wait)
}

// CreateContainerWithOptions creates a new LXC container
func CreateContainerWithOptions(ctx context.Context, opts CreateOptions) error {
	node, name, template, vmid := opts.Node, opts.Name, opts.Template, opts.VMID
	memory, disk, cores := opts.Memory, opts.Disk, opts.Cores
	password, sshKeys, storage, wait := opts.Password, opts.SSHKeys, opts.Storage, opts.Wait

	net0, err := buildNet0(opts.Network)
	if err != nil {
		return err
	}

	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}

	output.Info("Creating container %s...\n", name)

	// Resolve template if it's in short format
	resolvedTemplate, err := ResolveTemplateOnNode(ctx, template, node)
	if err != nil {
		return fmt.Errorf("failed to resolve template: %w", err)
	}
	template = resolvedTemplate.Template

	// Use the node where the template is located, unless a specific node is provided
	if node == "" {
		node = resolvedTemplate.Node
		output.Info("Using node: %s (where template is located)\n", node)
	} else {
		// The user chose a node, so don't quietly create the container on
		// another one. With shared template storage the template is found on
		// every node, so this only triggers when it really is not there.
		if node != resolvedTemplate.Node {
			return fmt.Errorf("template %s is not on node %s (it is on %s); download it there with 'prox ct templates pull %s --node %s', or leave out --node to use %s",
				template, node, resolvedTemplate.Node, template, node, resolvedTemplate.Node)
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
		names, err := client.GetStorageNames(ctx, node, "rootdir")
		if err != nil {
			names = nil // fall back to local-lvm, as before
		}
		storage = pickRootfsStorage(names)
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
		"net0":         net0,
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

	if err := cmdutil.FinishTask(ctx, client, node, taskID, wait,
		fmt.Sprintf("Container %s (ID: %d) created on node %s", name, vmid, node)); err != nil {
		return fmt.Errorf("container creation failed: %w", err)
	}
	if !wait {
		return nil
	}

	if opts.Start {
		// The cluster listing fetched while resolving the template does not
		// have the new container in it
		client.ClearClusterResourcesCache()
		// By ID: the name may be shared with another container
		if err := StartContainerWithWait(ctx, strconv.Itoa(vmid), true); err != nil {
			return fmt.Errorf("container created but failed to start: %w", err)
		}
		return nil
	}
	output.Info("Tip: Use 'prox ct start %s' to start the container\n", name)

	return nil
}

// defaultRootfsStorage is used when it exists, matching earlier versions
const defaultRootfsStorage = "local-lvm"

// pickRootfsStorage chooses where to put a container's root disk when the
// user didn't say: local-lvm if available (as before), otherwise the first
// storage that can hold containers. With no information it returns local-lvm.
func pickRootfsStorage(available []string) string {
	if len(available) == 0 || slices.Contains(available, defaultRootfsStorage) {
		return defaultRootfsStorage
	}
	return available[0]
}

// NetworkOptions configures a container's first network interface (eth0)
type NetworkOptions struct {
	Bridge  string // default vmbr0
	IP      string // "dhcp" (default) or an address in CIDR form, e.g. 10.0.0.5/24
	Gateway string // only with a static IP
	VLAN    int    // 0: untagged
}

// buildNet0 returns the Proxmox net0 value for opts
func buildNet0(opts NetworkOptions) (string, error) {
	bridge := opts.Bridge
	if bridge == "" {
		bridge = "vmbr0"
	}
	if strings.ContainsAny(bridge, ",= ") {
		return "", fmt.Errorf("invalid bridge name %q", bridge)
	}

	ip := opts.IP
	if ip == "" {
		ip = "dhcp"
	}
	if ip != "dhcp" {
		if _, _, err := net.ParseCIDR(ip); err != nil {
			return "", fmt.Errorf("invalid --ip %q: use dhcp or an address with prefix length, e.g. 10.0.0.5/24", ip)
		}
	}

	net0 := fmt.Sprintf("name=eth0,bridge=%s,ip=%s", bridge, ip)

	if opts.Gateway != "" {
		if ip == "dhcp" {
			return "", fmt.Errorf("--gateway needs a static --ip")
		}
		if net.ParseIP(opts.Gateway) == nil {
			return "", fmt.Errorf("invalid --gateway %q", opts.Gateway)
		}
		net0 += ",gw=" + opts.Gateway
	}

	if opts.VLAN != 0 {
		if opts.VLAN < 1 || opts.VLAN > 4094 {
			return "", fmt.Errorf("invalid --vlan %d: must be between 1 and 4094", opts.VLAN)
		}
		net0 += fmt.Sprintf(",tag=%d", opts.VLAN)
	}
	return net0, nil
}
