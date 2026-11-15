package lookup

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/scotty-c/prox/pkg/apierr"
	c "github.com/scotty-c/prox/pkg/client"
)

type fakeClient struct {
	c.ProxmoxClientInterface
	resources []c.Resource
}

func (f *fakeClient) GetClusterResources(context.Context) ([]c.Resource, error) {
	return f.resources, nil
}

func id(i int) *int { return &i }

var cluster = &fakeClient{resources: []c.Resource{
	{Type: "node", Node: "pve1"},
	{Type: "qemu", Node: "pve1", VMID: id(100), Name: "web"},
	{Type: "qemu", Node: "pve2", VMID: id(200), Name: "web"},
	{Type: "qemu", Node: "pve2", VMID: id(201), Name: "api"},
	{Type: "lxc", Node: "pve1", VMID: id(300), Name: "db"},
	{Type: "lxc", Node: "pve2", VMID: id(301), Name: "cache"},
	{Type: "lxc", Node: "pve2", VMID: id(302), Name: "cache"},
	{Type: "lxc", Node: "pve1", VMID: id(400)}, // no name
}}

func TestFindKeepsVMsAndContainersApart(t *testing.T) {
	ctx := context.Background()
	if r, err := Find(ctx, cluster, VM, "api", ""); err != nil || *r.VMID != 201 {
		t.Errorf("VM api: %+v, %v", r, err)
	}
	if _, err := Find(ctx, cluster, VM, "db", ""); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("a container is not a VM: %v", err)
	}
	if r, err := Find(ctx, cluster, Container, "db", ""); err != nil || *r.VMID != 300 {
		t.Errorf("container db: %+v, %v", r, err)
	}
	if _, err := Find(ctx, cluster, Container, "api", ""); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("a VM is not a container: %v", err)
	}
}

func TestFindByIDAndByNode(t *testing.T) {
	ctx := context.Background()
	if r, err := Find(ctx, cluster, VM, "200", ""); err != nil || r.Name != "web" || r.Node != "pve2" {
		t.Errorf("by ID: %+v, %v", r, err)
	}
	if r, err := Find(ctx, cluster, VM, "web", "pve2"); err != nil || *r.VMID != 200 {
		t.Errorf("--node picks between same-named VMs: %+v, %v", r, err)
	}
	if _, err := Find(ctx, cluster, VM, "200", "pve1"); err == nil || !strings.Contains(err.Error(), "not found on node 'pve1'") {
		t.Errorf("wrong node: %v", err)
	}
}

func TestAmbiguousNamesNameTheCandidatesAndTheRightHint(t *testing.T) {
	ctx := context.Background()
	_, err := Find(ctx, cluster, VM, "web", "")
	if !errors.Is(err, apierr.ErrAmbiguous) || errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("kind: %v", err)
	}
	for _, want := range []string{"more than one VM is named 'web'", "ID 100 on node pve1", "ID 200 on node pve2", "or --node to pick the node"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("VM message should contain %q:\n%v", want, err)
		}
	}

	_, err = Find(ctx, cluster, Container, "cache", "")
	if !errors.Is(err, apierr.ErrAmbiguous) {
		t.Fatalf("kind: %v", err)
	}
	if !strings.Contains(err.Error(), "more than one container is named 'cache'") || strings.Contains(err.Error(), "--node") {
		t.Errorf("container message should not offer --node:\n%v", err)
	}
}

func TestNotFoundSuggestsSimilarNames(t *testing.T) {
	_, err := Find(context.Background(), cluster, VM, "we", "")
	if !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("kind: %v", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "VM 'we' not found") || !strings.Contains(msg, "Did you mean one of these?") ||
		!strings.Contains(msg, "prox vm list") || strings.Count(msg, "• web") != 1 {
		t.Errorf("message:\n%s", msg)
	}

	_, err = Find(context.Background(), cluster, Container, "zzzzzzzzzzzz", "")
	if !strings.Contains(err.Error(), "prox ct list") || !strings.Contains(err.Error(), "containers") {
		t.Errorf("container message:\n%v", err)
	}
}

func TestCheckIDFree(t *testing.T) {
	ctx := context.Background()
	if err := CheckIDFree(ctx, cluster, 999); err != nil {
		t.Errorf("free ID: %v", err)
	}
	err := CheckIDFree(ctx, cluster, 100)
	if err == nil || !strings.Contains(err.Error(), "ID 100 is already in use by VM 'web' on node pve1") {
		t.Errorf("VM: %v", err)
	}
	err = CheckIDFree(ctx, cluster, 300)
	if err == nil || !strings.Contains(err.Error(), "already in use by container 'db'") {
		t.Errorf("container ID is also taken: %v", err)
	}
}
