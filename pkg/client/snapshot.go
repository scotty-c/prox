package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
)

// Guest types used in API paths
const (
	GuestVM        = "qemu"
	GuestContainer = "lxc"
)

// Snapshot is a VM or container snapshot
type Snapshot struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parent      string `json:"parent,omitempty"`
	// SnapTime is the creation time as a Unix timestamp (0 if unknown)
	SnapTime int64 `json:"snaptime,omitempty"`
	// VMState reports whether the VM's memory was saved (VMs only)
	VMState bool `json:"vmstate,omitempty"`
}

// snapshotNamePattern matches the names Proxmox accepts: a letter followed by
// letters, digits, '-' or '_', up to 40 characters
var snapshotNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,39}$`)

// ValidateSnapshotName checks a snapshot name before sending it to Proxmox
func ValidateSnapshotName(name string) error {
	if name == "current" {
		return fmt.Errorf("'current' is reserved by Proxmox and can't be used as a snapshot name")
	}
	if !snapshotNamePattern.MatchString(name) {
		return fmt.Errorf("invalid snapshot name %q: start with a letter and use only letters, digits, '-' and '_' (up to 40 characters)", name)
	}
	return nil
}

func snapshotPath(node, guestType string, vmid int) (string, error) {
	if guestType != GuestVM && guestType != GuestContainer {
		return "", fmt.Errorf("unknown guest type %q", guestType)
	}
	return fmt.Sprintf("/nodes/%s/%s/%d/snapshot", url.PathEscape(node), guestType, vmid), nil
}

// ListSnapshots returns the snapshots of a VM or container, oldest first.
// Proxmox's "current" placeholder (the running state) is left out.
func (c *ProxmoxClient) ListSnapshots(ctx context.Context, node, guestType string, vmid int) ([]Snapshot, error) {
	path, err := snapshotPath(node, guestType, vmid)
	if err != nil {
		return nil, err
	}
	raw, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Parent      string `json:"parent"`
			SnapTime    int64  `json:"snaptime"`
			VMState     int    `json:"vmstate"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse snapshot list response: %w", err)
	}

	snapshots := []Snapshot{}
	for _, s := range resp.Data {
		if s.Name == "current" {
			continue
		}
		snapshots = append(snapshots, Snapshot{
			Name:        s.Name,
			Description: s.Description,
			Parent:      s.Parent,
			SnapTime:    s.SnapTime,
			VMState:     s.VMState == 1,
		})
	}
	sort.SliceStable(snapshots, func(i, j int) bool { return snapshots[i].SnapTime < snapshots[j].SnapTime })
	return snapshots, nil
}

// CreateSnapshot starts creating a snapshot and returns the task ID.
// vmState saves the VM's memory too (VMs only; ignored for containers).
func (c *ProxmoxClient) CreateSnapshot(ctx context.Context, node, guestType string, vmid int, name, description string, vmState bool) (string, error) {
	if err := ValidateSnapshotName(name); err != nil {
		return "", err
	}
	path, err := snapshotPath(node, guestType, vmid)
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"snapname": name}
	if description != "" {
		body["description"] = description
	}
	if vmState && guestType == GuestVM {
		body["vmstate"] = ProxmoxBoolTrue
	}
	return requestData[string](ctx, c, "POST", path, body, "create snapshot")
}

// RollbackSnapshot starts rolling back to a snapshot and returns the task ID
func (c *ProxmoxClient) RollbackSnapshot(ctx context.Context, node, guestType string, vmid int, name string) (string, error) {
	path, err := snapshotPath(node, guestType, vmid)
	if err != nil {
		return "", err
	}
	return requestData[string](ctx, c, "POST", path+"/"+url.PathEscape(name)+"/rollback", nil, "rollback snapshot")
}

// DeleteSnapshot starts deleting a snapshot and returns the task ID
func (c *ProxmoxClient) DeleteSnapshot(ctx context.Context, node, guestType string, vmid int, name string) (string, error) {
	path, err := snapshotPath(node, guestType, vmid)
	if err != nil {
		return "", err
	}
	return requestData[string](ctx, c, "DELETE", path+"/"+url.PathEscape(name), nil, "delete snapshot")
}
