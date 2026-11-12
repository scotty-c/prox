package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

func guestPath(node, guestType string, vmid int) (string, error) {
	if guestType != GuestVM && guestType != GuestContainer {
		return "", fmt.Errorf("unknown guest type %q", guestType)
	}
	return fmt.Sprintf("/nodes/%s/%s/%d", url.PathEscape(node), guestType, vmid), nil
}

// RebootGuest asks a VM or container to reboot cleanly and returns the task ID
func (c *ProxmoxClient) RebootGuest(ctx context.Context, node, guestType string, vmid int) (string, error) {
	path, err := guestPath(node, guestType, vmid)
	if err != nil {
		return "", err
	}
	return requestData[string](ctx, c, "POST", path+"/status/reboot", nil, "reboot")
}

// CloneContainer clones a container and returns the task ID. full makes a
// full copy instead of a linked clone (needed unless the source is a template).
func (c *ProxmoxClient) CloneContainer(ctx context.Context, node string, vmid int, newid int, hostname string, full bool) (string, error) {
	path, err := guestPath(node, GuestContainer, vmid)
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"newid": newid}
	if hostname != "" {
		body["hostname"] = hostname
	}
	if full {
		body["full"] = ProxmoxBoolTrue
	}
	return requestData[string](ctx, c, "POST", path+"/clone", body, "clone container")
}

// MigrateContainer moves a container to another node and returns the task ID.
// A running container can only be moved with restart, which stops it, moves
// it and starts it again on the target.
func (c *ProxmoxClient) MigrateContainer(ctx context.Context, node string, vmid int, targetNode string, restart bool) (string, error) {
	path, err := guestPath(node, GuestContainer, vmid)
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"target": targetNode}
	if restart {
		body["restart"] = ProxmoxBoolTrue
	}
	return requestData[string](ctx, c, "POST", path+"/migrate", body, "migrate container")
}

// UpdateContainer changes a container's configuration. Proxmox applies this
// immediately, so there is no task to wait for.
func (c *ProxmoxClient) UpdateContainer(ctx context.Context, node string, vmid int, config map[string]interface{}) error {
	path, err := guestPath(node, GuestContainer, vmid)
	if err != nil {
		return err
	}
	raw, err := c.makeRequest(ctx, "PUT", path+"/config", config)
	if err != nil {
		return err
	}
	var resp APIResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("failed to parse update container response: %w", err)
	}
	return nil
}

// ResizeContainerDisk grows a container volume (e.g. "rootfs") by size
// (e.g. "+10G") and returns the task ID
func (c *ProxmoxClient) ResizeContainerDisk(ctx context.Context, node string, vmid int, disk, size string) (string, error) {
	path, err := guestPath(node, GuestContainer, vmid)
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"disk": disk, "size": size}
	return requestData[string](ctx, c, "PUT", path+"/resize", body, "resize container disk")
}

// Appliance is a container template that Proxmox can download (aplinfo)
type Appliance struct {
	Template    string `json:"template"` // file name, e.g. debian-12-standard_12.7-1_amd64.tar.zst
	OS          string `json:"os,omitempty"`
	Version     string `json:"version,omitempty"`
	Section     string `json:"section,omitempty"`
	Description string `json:"headline,omitempty"`
}

// GetAvailableTemplates lists the templates a node can download
func (c *ProxmoxClient) GetAvailableTemplates(ctx context.Context, node string) ([]Appliance, error) {
	raw, err := c.makeRequest(ctx, "GET", fmt.Sprintf("/nodes/%s/aplinfo", url.PathEscape(node)), nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []Appliance `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse aplinfo response: %w", err)
	}
	return resp.Data, nil
}

// DownloadTemplate starts downloading an appliance template to storage and
// returns the task ID
func (c *ProxmoxClient) DownloadTemplate(ctx context.Context, node, storage, template string) (string, error) {
	body := map[string]interface{}{"storage": storage, "template": template}
	return requestData[string](ctx, c, "POST", fmt.Sprintf("/nodes/%s/aplinfo", url.PathEscape(node)), body, "download template")
}
