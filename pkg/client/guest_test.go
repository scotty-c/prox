package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestAvailableTemplates(t *testing.T) {
	var requests []recordedRequest
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := recordedRequest{Method: r.Method, Path: r.URL.Path}
		if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
			_ = json.Unmarshal(raw, &req.Body)
		}
		requests = append(requests, req)
		if r.Method == http.MethodGet {
			w.Write([]byte(`{"data":[{"template":"debian-12-standard_12.7-1_amd64.tar.zst","section":"system","headline":"Debian 12"}]}`))
			return
		}
		w.Write([]byte(`{"data":"UPID:pve:1:2:3:download:0:root@pam:"}`))
	}))
	defer srv.Close()
	c, _ := NewClientWithTLS(srv.URL, "root@pam!t", "s", TLSOptions{Insecure: true})
	ctx := context.Background()

	apps, err := c.GetAvailableTemplates(ctx, "pve")
	if err != nil || len(apps) != 1 || apps[0].Template != "debian-12-standard_12.7-1_amd64.tar.zst" || apps[0].Description != "Debian 12" {
		t.Fatalf("GetAvailableTemplates = %+v, %v", apps, err)
	}
	if _, err := c.DownloadTemplate(ctx, "pve", "local", apps[0].Template); err != nil {
		t.Fatal(err)
	}
	want := recordedRequest{"POST", "/api2/json/nodes/pve/aplinfo", map[string]interface{}{"storage": "local", "template": "debian-12-standard_12.7-1_amd64.tar.zst"}}
	if !reflect.DeepEqual(requests[1], want) {
		t.Fatalf("download request = %+v, want %+v", requests[1], want)
	}
}
