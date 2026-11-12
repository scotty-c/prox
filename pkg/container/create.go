package container

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strings"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/output"
)

// CreateContainer creates a new LXC container and waits for it to be created
func CreateContainer(node, name, template string, vmid int, memory, disk int, cores int, password, sshKeys, storage string) error {
	return CreateContainerWithWait(node, name, template, vmid, memory, disk, cores, password, sshKeys, storage, true)
}

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
}

// CreateContainerWithWait creates a new LXC container, optionally returning as
// soon as the creation task has started
func CreateContainerWithWait(node, name, template string, vmid int, memory, disk int, cores int, password, sshKeys, storage string, wait bool) error {
	return CreateContainerWithOptions(CreateOptions{
		Node: node, Name: name, Template: template, VMID: vmid,
		Memory: memory, Disk: disk, Cores: cores,
		Password: password, SSHKeys: sshKeys, Storage: storage, Wait: wait,
	})
}

// CreateContainerWithOptions creates a new LXC container
func CreateContainerWithOptions(opts CreateOptions) error {
	node, name, template, vmid := opts.Node, opts.Name, opts.Template, opts.VMID
	memory, disk, cores := opts.Memory, opts.Disk, opts.Cores
	password, sshKeys, storage, wait := opts.Password, opts.SSHKeys, opts.Storage, opts.Wait

	net0, err := buildNet0(opts.Network)
	if err != nil {
		return err
	}

	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}

	output.Info("Creating container %s...\n", name)

	// Resolve template if it's in short format
	resolvedTemplate, err := ResolveTemplateOnNode(template, node)
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

	output.Result("Task started: %s\n", taskID)
	if !wait {
		output.Info("Tip: Follow progress with 'prox logs --follow %s'\n", taskID)
		return nil
	}

	// Wait for task completion
	err = waitForTask(ctx, client, node, taskID)
	if err != nil {
		return fmt.Errorf("container creation failed: %w", err)
	}

	output.Result("Container %s (ID: %d) created successfully on node %s\n", name, vmid, node)
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
