package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestForceStopUsesStopEndpoint(t *testing.T) {
	var paths []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		paths = append(paths, r.URL.Path)
		w.Write([]byte(`{"data":"UPID:pve:1:2:3:qmstop:100:root@pam:"}`))
	}))
	defer srv.Close()

	// API token auth avoids a ticket request in this test
	c, _ := NewClientWithTLS(srv.URL, "root@pam!t", "s", TLSOptions{Insecure: true})
	ctx := context.Background()
	if upid, err := c.ForceStopVM(ctx, "pve", 100); err != nil || upid == "" {
		t.Fatalf("ForceStopVM = %q, %v", upid, err)
	}
	if _, err := c.ForceStopContainer(ctx, "pve", 101); err != nil {
		t.Fatal(err)
	}
	want := []string{"/api2/json/nodes/pve/qemu/100/status/stop", "/api2/json/nodes/pve/lxc/101/status/stop"}
	if len(paths) != 2 || paths[0] != want[0] || paths[1] != want[1] {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}
