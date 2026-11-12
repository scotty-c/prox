package vm

import (
	"context"
	"strings"
	"testing"

	c "github.com/scotty-c/prox/pkg/client"
)

// fakeClient implements only GetClusterResources; any other call panics.
type fakeClient struct {
	c.ProxmoxClientInterface
	resources []c.Resource
}

func (f *fakeClient) GetClusterResources(ctx context.Context) ([]c.Resource, error) {
	return f.resources, nil
}

func intPtr(i int) *int { return &i }

func TestFindByNameOrIDOnNode(t *testing.T) {
	client := &fakeClient{resources: []c.Resource{
		{Type: "node", Node: "pve1"},
		{Type: "qemu", Node: "pve1", VMID: intPtr(100), Name: "web"},
		{Type: "qemu", Node: "pve2", VMID: intPtr(200), Name: "web"},
		{Type: "lxc", Node: "pve2", VMID: intPtr(300), Name: "db"},
		{Type: "qemu", Node: "pve2", VMID: intPtr(201), Name: "api"},
	}}
	ctx := context.Background()

	tests := []struct {
		nameOrID string
		node     string
		wantID   int
		wantErr  string
	}{
		{"web", "", 100, ""},     // no node: first match (existing behaviour)
		{"web", "pve2", 200, ""}, // node filter picks the VM on that node
		{"200", "pve2", 200, ""},
		{"200", "pve1", 0, "not found on node 'pve1'"},
		{"api", "pve1", 0, "not found on node 'pve1'"},
		{"db", "", 0, "not found"}, // containers are not VMs
		{"100%s", "", 0, "VM '100%s' not found"},
	}
	for _, tt := range tests {
		t.Run(tt.nameOrID+"@"+tt.node, func(t *testing.T) {
			got, err := FindByNameOrIDOnNode(ctx, client, tt.nameOrID, tt.node)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.ID != tt.wantID {
				t.Fatalf("ID = %d, want %d", got.ID, tt.wantID)
			}
		})
	}
}
