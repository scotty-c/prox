package client

import (
	"context"
	"reflect"
	"testing"
)

func TestDiskKeysFindsEveryDiskInOrder(t *testing.T) {
	config := map[string]interface{}{
		"scsi10": "x", "scsi2": "x", "scsi0": "x", "virtio15": "x", "sata5": "x", "ide2": "x",
		"net0": "x", "efidisk0": "x", "tpmstate0": "x", "scsihw": "virtio-scsi-pci", "memory": 2048,
	}
	want := []string{"ide2", "sata5", "scsi0", "scsi2", "scsi10", "virtio15"}
	if got := DiskKeys(config); !reflect.DeepEqual(got, want) {
		t.Fatalf("DiskKeys = %v, want %v", got, want)
	}
}

func TestNetKeysSortNumerically(t *testing.T) {
	config := map[string]interface{}{"net10": "x", "net2": "x", "net0": "x", "netmask": "x", "scsi0": "x"}
	want := []string{"net0", "net2", "net10"}
	if got := NetKeys(config); !reflect.DeepEqual(got, want) {
		t.Fatalf("NetKeys = %v, want %v", got, want)
	}
}

func TestConfigNumber(t *testing.T) {
	cases := []struct {
		in   interface{}
		want float64
		ok   bool
	}{
		{float64(2048), 2048, true}, {"2048", 2048, true}, {" 4 ", 4, true},
		{"abc", 0, false}, {nil, 0, false}, {true, 0, false},
	}
	for _, c := range cases {
		if got, ok := ConfigNumber(c.in); got != c.want || ok != c.ok {
			t.Errorf("ConfigNumber(%#v) = %v, %v; want %v, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestConfigFlag(t *testing.T) {
	cases := []struct {
		in       interface{}
		on, isOK bool
	}{
		{float64(1), true, true}, {float64(0), false, true}, {true, true, true},
		{"1", true, true}, {"0", false, true}, {"enabled=1", true, true},
		{"enabled=0,fstrim_cloned_disks=1", false, true},
		{"1,fstrim_cloned_disks=1", true, true},
		{"fstrim_cloned_disks=1,enabled=1", true, true},
		{"virtio", false, false}, {nil, false, false},
	}
	for _, c := range cases {
		if on, ok := ConfigFlag(c.in); on != c.on || ok != c.isOK {
			t.Errorf("ConfigFlag(%#v) = %v, %v; want %v, %v", c.in, on, ok, c.on, c.isOK)
		}
	}
}

func TestVolumeSizeLookupIsExact(t *testing.T) {
	c := &ProxmoxClient{}
	cache := map[string][]interface{}{
		"pve1:local-lvm": {
			map[string]interface{}{"volid": "local-lvm:vm-100-disk-10", "size": float64(100)},
			map[string]interface{}{"volid": "local-lvm:vm-100-disk-1", "size": float64(1)},
		},
	}
	got, err := c.getStorageVolumeSizeWithCache(context.Background(), "pve1", "local-lvm", "vm-100-disk-1", cache)
	if err != nil || got != 1 {
		t.Fatalf("size = %d, %v; want 1 (not disk-10's 100)", got, err)
	}
	if _, err := c.getStorageVolumeSizeWithCache(context.Background(), "pve1", "local-lvm", "vm-100-disk-", cache); err == nil {
		t.Fatal("a partial volume name must not match")
	}
}
