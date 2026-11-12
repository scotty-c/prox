package client

import (
	"context"
	"reflect"
	"testing"
)

func TestGuestOperationRequests(t *testing.T) {
	var requests []recordedRequest
	c := newSnapshotServer(t, &requests) // records every request; returns a UPID for non-GET
	ctx := context.Background()

	if _, err := c.RebootGuest(ctx, "pve", GuestVM, 100); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RebootGuest(ctx, "pve", GuestContainer, 101); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CloneContainer(ctx, "pve", 101, 201, "web-copy", true); err != nil {
		t.Fatal(err)
	}
	if _, err := c.MigrateContainer(ctx, "pve", 101, "pve2", true); err != nil {
		t.Fatal(err)
	}
	if err := c.UpdateContainer(ctx, "pve", 101, map[string]interface{}{"cores": 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ResizeContainerDisk(ctx, "pve", 101, "rootfs", "+10G"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RebootGuest(ctx, "pve", "bogus", 1); err == nil {
		t.Fatal("expected error for unknown guest type")
	}

	want := []recordedRequest{
		{"POST", "/api2/json/nodes/pve/qemu/100/status/reboot", nil},
		{"POST", "/api2/json/nodes/pve/lxc/101/status/reboot", nil},
		{"POST", "/api2/json/nodes/pve/lxc/101/clone", map[string]interface{}{"newid": float64(201), "hostname": "web-copy", "full": float64(1)}},
		{"POST", "/api2/json/nodes/pve/lxc/101/migrate", map[string]interface{}{"target": "pve2", "restart": float64(1)}},
		{"PUT", "/api2/json/nodes/pve/lxc/101/config", map[string]interface{}{"cores": float64(2)}},
		{"PUT", "/api2/json/nodes/pve/lxc/101/resize", map[string]interface{}{"disk": "rootfs", "size": "+10G"}},
	}
	if !reflect.DeepEqual(requests, want) {
		t.Fatalf("requests:\n got %+v\nwant %+v", requests, want)
	}
}
