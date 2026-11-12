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

func TestValidateSnapshotName(t *testing.T) {
	for _, ok := range []string{"before-upgrade", "snap_1", "A"} {
		if err := ValidateSnapshotName(ok); err != nil {
			t.Errorf("ValidateSnapshotName(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "1st", "has space", "current", "a.b", "x0123456789012345678901234567890123456789"} {
		if ValidateSnapshotName(bad) == nil {
			t.Errorf("ValidateSnapshotName(%q) should fail", bad)
		}
	}
}

type recordedRequest struct {
	Method, Path string
	Body         map[string]interface{}
}

func newSnapshotServer(t *testing.T, requests *[]recordedRequest) *ProxmoxClient {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := recordedRequest{Method: r.Method, Path: r.URL.Path}
		if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
			_ = json.Unmarshal(raw, &req.Body)
		}
		*requests = append(*requests, req)
		if r.Method == http.MethodGet {
			w.Write([]byte(`{"data":[
				{"name":"current","parent":"second","running":1},
				{"name":"second","parent":"first","snaptime":1700000200,"description":"later"},
				{"name":"first","snaptime":1700000100,"vmstate":1}
			]}`))
			return
		}
		w.Write([]byte(`{"data":"UPID:pve:1:2:3:qmsnapshot:100:root@pam:"}`))
	}))
	t.Cleanup(srv.Close)
	c, err := NewClientWithTLS(srv.URL, "root@pam!t", "s", TLSOptions{Insecure: true})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestListSnapshots(t *testing.T) {
	var requests []recordedRequest
	c := newSnapshotServer(t, &requests)

	got, err := c.ListSnapshots(context.Background(), "pve", GuestVM, 100)
	if err != nil {
		t.Fatal(err)
	}
	want := []Snapshot{
		{Name: "first", SnapTime: 1700000100, VMState: true},
		{Name: "second", Parent: "first", SnapTime: 1700000200, Description: "later"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if requests[0].Path != "/api2/json/nodes/pve/qemu/100/snapshot" {
		t.Fatalf("path = %s", requests[0].Path)
	}
}

func TestSnapshotTasks(t *testing.T) {
	var requests []recordedRequest
	c := newSnapshotServer(t, &requests)
	ctx := context.Background()

	if _, err := c.CreateSnapshot(ctx, "pve", GuestVM, 100, "pre", "desc", true); err != nil {
		t.Fatal(err)
	}
	// vmstate is VM-only and must not be sent for containers
	if _, err := c.CreateSnapshot(ctx, "pve", GuestContainer, 101, "pre", "", true); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RollbackSnapshot(ctx, "pve", GuestContainer, 101, "pre"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DeleteSnapshot(ctx, "pve", GuestVM, 100, "pre"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateSnapshot(ctx, "pve", GuestVM, 100, "bad name", "", false); err == nil {
		t.Fatal("expected validation error")
	}
	if _, err := c.ListSnapshots(ctx, "pve", "bogus", 100); err == nil {
		t.Fatal("expected error for unknown guest type")
	}

	want := []recordedRequest{
		{"POST", "/api2/json/nodes/pve/qemu/100/snapshot", map[string]interface{}{"snapname": "pre", "description": "desc", "vmstate": float64(1)}},
		{"POST", "/api2/json/nodes/pve/lxc/101/snapshot", map[string]interface{}{"snapname": "pre"}},
		{"POST", "/api2/json/nodes/pve/lxc/101/snapshot/pre/rollback", nil},
		{"DELETE", "/api2/json/nodes/pve/qemu/100/snapshot/pre", nil},
	}
	if !reflect.DeepEqual(requests, want) {
		t.Fatalf("requests:\n got %+v\nwant %+v", requests, want)
	}
}
