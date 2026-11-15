package client

import (
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
