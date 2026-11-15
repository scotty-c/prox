package guest

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	c "github.com/scotty-c/prox/pkg/client"
)

func ptr[T any](v T) *T { return &v }

func TestFromResourceFillsBothTheHumanAndTheScriptFields(t *testing.T) {
	g := FromResource(c.Resource{
		VMID: ptr(100), Name: "web", Status: "running", Node: "pve1", Tags: "prod;web",
		CPU: ptr(0.025), MaxCPU: ptr(4), Mem: ptr(int64(1 << 30)), MaxMem: ptr(int64(2 << 30)),
		Disk: ptr(int64(5 << 30)), MaxDisk: ptr(int64(32 << 30)), Uptime: ptr(int64(90061)),
	})
	if g.ID != 100 || g.Name != "web" || g.Status != "running" || g.Node != "pve1" || g.Tags != "prod;web" {
		t.Errorf("identity: %+v", g)
	}
	if g.CPUs != 2 || g.CPUPercent != 2.5 || g.CPUCount != 4 {
		t.Errorf("cpu: cpus %d, percent %v, count %d", g.CPUs, g.CPUPercent, g.CPUCount)
	}
	if g.Uptime != "1d 1h 1m" || g.UptimeSeconds != 90061 {
		t.Errorf("uptime: %q, %d", g.Uptime, g.UptimeSeconds)
	}
	if len(g.TagList) != 2 || g.TagList[0] != "prod" || g.TagList[1] != "web" {
		t.Errorf("tag list: %v", g.TagList)
	}
	if g.IP != "N/A" || g.IPAddress != "" {
		t.Errorf("an address nobody looked up: ip %q, ip_address %q", g.IP, g.IPAddress)
	}
}

func TestFromResourceToleratesMissingFields(t *testing.T) {
	g := FromResource(c.Resource{VMID: ptr(101), Name: "db", Status: "stopped", Node: "pve1"})
	if g.MaxMemory != 0 || g.CPUs != 0 || g.Uptime != "" || g.TagList == nil || len(g.TagList) != 0 {
		t.Errorf("%+v", g)
	}
}

func TestCellText(t *testing.T) {
	g := Guest{CPUs: 12, Memory: 1 << 30, MaxMemory: 2 << 30, Uptime: "3d 0h 0m"}
	if g.CPUText() != "12%" || g.MemoryText() != "1.0 GB/2.0 GB (50.0%)" || g.UptimeText() != "3d 0h 0m" {
		t.Errorf("%q %q %q", g.CPUText(), g.MemoryText(), g.UptimeText())
	}
	empty := Guest{}
	if empty.MemoryText() != "N/A" || empty.UptimeText() != "N/A" || empty.CPUText() != "0%" {
		t.Errorf("%q %q %q", empty.MemoryText(), empty.UptimeText(), empty.CPUText())
	}
}

func TestLookupIPsOnlyAsksAboutRunningGuestsAndKeepsNAForTheRest(t *testing.T) {
	guests := []Guest{
		{ID: 1, Node: "pve1", Status: "running", IP: "N/A"},
		{ID: 2, Node: "pve1", Status: "stopped", IP: "N/A"},
		{ID: 3, Node: "pve2", Status: "running", IP: "N/A"}, // lookup fails
		{ID: 4, Node: "pve2", Status: "running", IP: "N/A"}, // nothing found
	}
	var asked int32
	LookupIPs(context.Background(), guests, func(_ context.Context, node string, id int) (string, error) {
		atomic.AddInt32(&asked, 1)
		switch id {
		case 1:
			return "10.0.0.1", nil
		case 3:
			return "", errors.New("agent not running")
		}
		return "", nil
	})
	if asked != 3 {
		t.Errorf("asked %d times, want 3 (the stopped guest is skipped)", asked)
	}
	if guests[0].IP != "10.0.0.1" || guests[0].IPAddress != "10.0.0.1" {
		t.Errorf("guest 1: %+v", guests[0])
	}
	for _, i := range []int{1, 2, 3} {
		if guests[i].IP != "N/A" || guests[i].IPAddress != "" {
			t.Errorf("guest %d should stay unknown: ip %q, ip_address %q", guests[i].ID, guests[i].IP, guests[i].IPAddress)
		}
	}
}
