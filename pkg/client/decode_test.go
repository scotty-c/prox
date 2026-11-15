package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// serve answers every API path from routes (path without /api2/json -> the
// JSON body to send) and logs in successfully
func serve(t *testing.T, routes map[string]string) *ProxmoxClient {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/access/ticket" {
			w.Write([]byte(`{"data":{"ticket":"t","CSRFPreventionToken":"c"}}`))
			return
		}
		body, ok := routes[strings.TrimPrefix(r.URL.Path, "/api2/json")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	c, err := NewClientWithTLS(srv.URL, "root@pam", "pw", TLSOptions{Insecure: true})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNextIDAcceptsNumberOrString(t *testing.T) {
	for body, want := range map[string]int{`{"data":"103"}`: 103, `{"data":104}`: 104} {
		c := serve(t, map[string]string{"/cluster/nextid": body})
		got, err := c.GetNextVMID(context.Background())
		if err != nil || got != want {
			t.Errorf("%s: got %d, %v; want %d", body, got, err, want)
		}
	}
	c := serve(t, map[string]string{"/cluster/nextid": `{"data":"soon"}`})
	if _, err := c.GetNextVMID(context.Background()); err == nil {
		t.Error("a non-numeric ID should be an error")
	}
}

func TestMissingTaskIDIsAnErrorNotAnEmptyTask(t *testing.T) {
	c := serve(t, map[string]string{
		"/nodes/pve1/qemu/100/status/start": `{"data":null}`,
		"/nodes/pve1/qemu/101/status/start": `{"data":"UPID:pve1:1:2:3:qmstart:101:root@pam:"}`,
	})
	if id, err := c.StartVM(context.Background(), "pve1", 100); err == nil {
		t.Errorf("null data returned task %q with no error", id)
	}
	if id, err := c.StartVM(context.Background(), "pve1", 101); err != nil || !strings.HasPrefix(id, "UPID:") {
		t.Errorf("got %q, %v", id, err)
	}
}

func TestMissingConfigIsAnError(t *testing.T) {
	c := serve(t, map[string]string{"/nodes/pve1/qemu/100/config": `{"data":null}`})
	if _, err := c.GetVMConfig(context.Background(), "pve1", 100); err == nil {
		t.Error("null config should be an error")
	}
}

func TestOptionalTaskIsEmptyWhenProxmoxReturnsNothing(t *testing.T) {
	c := serve(t, map[string]string{
		"/nodes/pve1/qemu/100/config": `{"data":null}`,
		"/nodes/pve1/qemu/101/config": `{"data":"UPID:pve1:1:2:3:qmconfig:101:root@pam:"}`,
	})
	if id, err := c.UpdateVM(context.Background(), "pve1", 100, map[string]interface{}{"cores": 2}); err != nil || id != "" {
		t.Errorf("no task: got %q, %v", id, err)
	}
	if id, err := c.UpdateVM(context.Background(), "pve1", 101, map[string]interface{}{"cores": 2}); err != nil || id == "" {
		t.Errorf("task: got %q, %v", id, err)
	}
}

func TestVersionNodesAndTaskStatusAreDecoded(t *testing.T) {
	c := serve(t, map[string]string{
		"/version":                        `{"data":{"version":"8.2.4","release":"8.2","repoid":"abc"}}`,
		"/nodes":                          `{"data":[{"node":"pve1","status":"online","type":"node","id":"node/pve1"},{"node":"pve2","status":"offline"}]}`,
		"/nodes/pve1/tasks/UPID:x/status": `{"data":{"upid":"UPID:x","status":"stopped","exitstatus":"OK","pid":42,"type":"qmstart"}}`,
	})
	ctx := context.Background()
	if v, err := c.GetVersion(ctx); err != nil || v.Version != "8.2.4" || v.RepoID != "abc" {
		t.Errorf("version = %+v, %v", v, err)
	}
	if n, err := c.GetNodes(ctx); err != nil || len(n) != 2 || n[1].Status != "offline" {
		t.Errorf("nodes = %+v, %v", n, err)
	}
	if task, err := c.GetTaskStatus(ctx, "pve1", "UPID:x"); err != nil || task.ExitCode != "OK" || task.PID != 42 {
		t.Errorf("task = %+v, %v", task, err)
	}
}

func TestStorageNamesSkipInactiveStorage(t *testing.T) {
	c := serve(t, map[string]string{"/nodes/pve1/storage": `{"data":[
		{"storage":"local","active":1},{"storage":"nfs","active":0},{"storage":"zfs"},{"storage":"ceph","active":true}]}`})
	got, err := c.GetStorageNames(context.Background(), "pve1", "rootdir")
	if err != nil || strings.Join(got, ",") != "local,zfs,ceph" {
		t.Errorf("got %v, %v; want local,zfs,ceph", got, err)
	}
}

func TestContainerTemplatesAreReadFromTemplateStorageOnly(t *testing.T) {
	c := serve(t, map[string]string{
		"/nodes/pve1/storage":               `{"data":[{"storage":"local","content":"iso,vztmpl"},{"storage":"lvm","content":"images"}]}`,
		"/nodes/pve1/storage/local/content": `{"data":[{"volid":"local:vztmpl/debian-12.tar.zst","size":123456789,"used":1}]}`,
	})
	got, err := c.GetContainerTemplates(context.Background(), "pve1")
	if err != nil || len(got) != 1 || got[0].VolID != "local:vztmpl/debian-12.tar.zst" || got[0].Size != 123456789 {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestVMIPPrefersTheGuestAgentThenCloudInit(t *testing.T) {
	c := serve(t, map[string]string{
		"/nodes/pve1/qemu/100/agent/network-get-interfaces": `{"data":{"result":[
			{"name":"lo","ip-addresses":[{"ip-address":"127.0.0.1","ip-address-type":"ipv4"}]},
			{"name":"eth0","ip-addresses":[{"ip-address":"10.0.0.7","ip-address-type":"ipv4"}]}]}}`,
		"/nodes/pve1/qemu/100/config": `{"data":{"ipconfig0":"ip=192.168.1.9/24"}}`,
		// 101: no agent, but a static cloud-init address
		"/nodes/pve1/qemu/101/config": `{"data":{"ipconfig0":"ip=192.168.1.9/24,gw=192.168.1.1"}}`,
		// 102: no agent and DHCP only
		"/nodes/pve1/qemu/102/config": `{"data":{"ipconfig0":"ip=dhcp"}}`,
	})
	ctx := context.Background()
	for vmid, want := range map[int]string{100: "10.0.0.7", 101: "192.168.1.9", 102: ""} {
		got, err := c.GetVMIP(ctx, "pve1", vmid)
		if err != nil || got != want {
			t.Errorf("VM %d: got %q, %v; want %q", vmid, got, err, want)
		}
	}
}

func TestUnknownIPIsEmptyNotASentinel(t *testing.T) {
	c := serve(t, map[string]string{})
	if ip, err := c.GetVMIP(context.Background(), "pve1", 1); ip != "" || err != nil {
		t.Errorf("VM: %q, %v", ip, err)
	}
	if ip, err := c.GetContainerIP(context.Background(), "pve1", 2); ip != "" || err != nil {
		t.Errorf("container: %q, %v", ip, err)
	}
}

func TestContainerIPComesFromItsInterfaces(t *testing.T) {
	c := serve(t, map[string]string{
		"/nodes/pve1/lxc/200/status/current": `{"data":{"status":"running"}}`,
		"/nodes/pve1/lxc/200/interfaces":     `{"data":[{"name":"lo","inet":"127.0.0.1/8"},{"name":"eth0","inet":"10.0.0.50/24"}]}`,
	})
	if ip, err := c.GetContainerIP(context.Background(), "pve1", 200); err != nil || ip != "10.0.0.50" {
		t.Errorf("got %q, %v", ip, err)
	}
}

func TestNodeIPSkipsLoopbackAndStripsThePrefix(t *testing.T) {
	c := serve(t, map[string]string{
		"/nodes/pve1/network": `{"data":[{"iface":"lo","address":"127.0.0.1"},{"iface":"vmbr0","cidr":"192.168.1.11/24"}]}`,
		"/nodes/pve2/network": `{"data":[{"iface":"lo","address":"127.0.0.1"}]}`,
	})
	if ip, err := c.GetNodeIP(context.Background(), "pve1"); err != nil || ip != "192.168.1.11" {
		t.Errorf("pve1: %q, %v", ip, err)
	}
	if ip, err := c.GetNodeIP(context.Background(), "pve2"); err != nil || ip != "" {
		t.Errorf("pve2: %q, %v", ip, err)
	}
}
