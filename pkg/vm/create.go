package vm

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/output"
)

// CreateOptions describes a VM to create from a cloud-init template
type CreateOptions struct {
	Name     string
	Template string // name or ID of the template VM to clone
	Node     string // only match the template on this node
	NewID    int    // 0: next free ID
	Full     bool   // full clone instead of linked

	Cores    int // 0: keep the template's
	MemoryMB int // 0: keep the template's
	DiskGB   int // grow the primary disk by this much

	IP      string // "", "dhcp" or CIDR, e.g. 10.0.0.20/24
	Gateway string
	CIUser  string
	SSHKeys string // public keys, one per line

	Start bool
	Wait  bool // wait for the VM to start (with Start)
}

// cloudInitConfig returns the config changes for opts (excluding disk size)
func cloudInitConfig(opts CreateOptions) (map[string]interface{}, error) {
	config := map[string]interface{}{}
	if opts.Cores > 0 {
		config["cores"] = opts.Cores
	}
	if opts.MemoryMB > 0 {
		config["memory"] = opts.MemoryMB
	}

	switch {
	case opts.IP == "" && opts.Gateway != "":
		return nil, fmt.Errorf("--gateway needs a static --ip")
	case opts.IP == "dhcp":
		if opts.Gateway != "" {
			return nil, fmt.Errorf("--gateway needs a static --ip")
		}
		config["ipconfig0"] = "ip=dhcp"
	case opts.IP != "":
		if _, _, err := net.ParseCIDR(opts.IP); err != nil {
			return nil, fmt.Errorf("invalid --ip %q: use dhcp or an address with prefix length, e.g. 10.0.0.20/24", opts.IP)
		}
		ipconfig := "ip=" + opts.IP
		if opts.Gateway != "" {
			if net.ParseIP(opts.Gateway) == nil {
				return nil, fmt.Errorf("invalid --gateway %q", opts.Gateway)
			}
			ipconfig += ",gw=" + opts.Gateway
		}
		config["ipconfig0"] = ipconfig
	}

	if opts.CIUser != "" {
		config["ciuser"] = opts.CIUser
	}
	if keys := strings.TrimSpace(opts.SSHKeys); keys != "" {
		config["sshkeys"] = encodeSSHKeys(keys)
	}
	return config, nil
}

// encodeSSHKeys URL-encodes public keys the way the Proxmox API expects for
// the sshkeys option: every byte except letters, digits and -_.~ is
// percent-encoded, and spaces become %20 (not +)
func encodeSSHKeys(keys string) string {
	var b strings.Builder
	for i := 0; i < len(keys); i++ {
		ch := keys[i]
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || strings.IndexByte("-_.~", ch) != -1 {
			b.WriteByte(ch)
		} else {
			fmt.Fprintf(&b, "%%%02X", ch)
		}
	}
	return b.String()
}

// idFree returns an error if a VM or container already uses id
func idFree(ctx context.Context, client c.ProxmoxClientInterface, id int) error {
	resources, err := client.GetClusterResources(ctx)
	if err != nil {
		return fmt.Errorf("failed to get cluster resources: %w", err)
	}
	for _, r := range resources {
		if (r.Type == "qemu" || r.Type == "lxc") && r.VMID != nil && *r.VMID == id {
			return fmt.Errorf("ID %d is already in use by '%s' on node %s", id, r.Name, r.Node)
		}
	}
	return nil
}

// CreateFromTemplate clones a cloud-init template and configures the new VM
func CreateFromTemplate(opts CreateOptions) error {
	if err := ValidateNewVMName(opts.Name); err != nil {
		return err
	}
	config, err := cloudInitConfig(opts)
	if err != nil {
		return err
	}

	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		output.ClientError(err)
		return fmt.Errorf("failed to create client: %w", err)
	}

	tmpl, err := FindByNameOrIDOnNode(ctx, client, opts.Template, opts.Node)
	if err != nil {
		return fmt.Errorf("failed to find template: %w", err)
	}

	newID := opts.NewID
	if newID == 0 {
		if newID, err = client.GetNextVMID(ctx); err != nil {
			return fmt.Errorf("failed to get next ID: %w", err)
		}
	} else if err := idFree(ctx, client, newID); err != nil {
		return err
	}

	start := time.Now()
	output.Info("Cloning %s (ID: %d) to %s (ID: %d) on node %s...\n", tmpl.Name, tmpl.ID, opts.Name, newID, tmpl.Node)
	taskID, err := client.CloneVM(ctx, tmpl.Node, tmpl.ID, newID, opts.Name, opts.Full)
	if err != nil {
		return fmt.Errorf("failed to clone template: %w", err)
	}
	if err := waitForTask(ctx, client, tmpl.Node, taskID); err != nil {
		return fmt.Errorf("clone failed: %w", err)
	}

	// The clone exists from here on; say which VM to clean up if a later step fails
	fail := func(step string, err error) error {
		return fmt.Errorf("%s failed: %w\n\nVM %s (ID: %d) was created but is not fully configured; fix it with 'prox vm edit' or remove it with 'prox vm delete %d'", step, err, opts.Name, newID, newID)
	}

	if len(config) > 0 {
		output.Info("Applying configuration...\n")
		taskID, err := client.UpdateVM(ctx, tmpl.Node, newID, config)
		if err != nil {
			return fail("configuration", err)
		}
		if taskID != "" {
			if err := waitForTask(ctx, client, tmpl.Node, taskID); err != nil {
				return fail("configuration", err)
			}
		}
	}

	if opts.DiskGB > 0 {
		vm := &VirtualMachine{ID: newID, Node: tmpl.Node, Client: client}
		disk, err := vm.GetPrimaryDisk(ctx)
		if err != nil {
			return fail("disk resize", err)
		}
		output.Info("Growing %s by %d GB...\n", disk, opts.DiskGB)
		taskID, err := client.ResizeDisk(ctx, tmpl.Node, newID, disk, fmt.Sprintf("+%dG", opts.DiskGB))
		if err != nil {
			return fail("disk resize", err)
		}
		if err := waitForTask(ctx, client, tmpl.Node, taskID); err != nil {
			return fail("disk resize", err)
		}
	}

	output.Result("VM %s (ID: %d) created on node %s in %s\n", opts.Name, newID, tmpl.Node, time.Since(start).Round(time.Millisecond))

	if opts.Start {
		if err := StartVMWithWait(ctx, newID, tmpl.Node, opts.Wait, client); err != nil {
			return fmt.Errorf("VM created but failed to start: %w", err)
		}
	} else {
		output.Info("Tip: Start it with 'prox vm start %s'\n", opts.Name)
	}
	return nil
}

// ValidateNewVMName checks a name for a new VM. Proxmox requires a valid DNS
// name: letters, digits, '-' and '.', not starting or ending with '-'.
func ValidateNewVMName(name string) error {
	if name == "" {
		return fmt.Errorf("VM name cannot be empty")
	}
	if len(name) > 63 {
		return fmt.Errorf("VM name %q is too long (max 63 characters)", name)
	}
	for i, ch := range name {
		ok := ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '.'
		if !ok || (ch == '-' && (i == 0 || i == len(name)-1)) {
			return fmt.Errorf("invalid VM name %q: use letters, digits, '-' and '.', not starting or ending with '-'", name)
		}
	}
	return nil
}
