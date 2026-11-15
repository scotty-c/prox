package vm

import "testing"

func TestPrimaryDisk(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]interface{}
		want   string
		ok     bool
	}{
		{"boot order wins", map[string]interface{}{
			"boot": "order=virtio0;ide2;net0", "scsi0": "local:vm-1-disk-1,size=8G", "virtio0": "local:vm-1-disk-0,size=32G",
			"ide2": "local:iso/x.iso,media=cdrom"}, "virtio0", true},
		{"cdrom and cloud-init are skipped", map[string]interface{}{
			"ide0": "local:iso/x.iso,media=cdrom", "ide2": "local-lvm:vm-9-cloudinit,media=cdrom", "scsi5": "local-lvm:vm-9-disk-0,size=20G"}, "scsi5", true},
		{"boot order ignores non-disks", map[string]interface{}{
			"boot": "order=ide2;net0;scsi1", "ide2": "local:iso/x.iso,media=cdrom", "scsi1": "local-lvm:vm-9-disk-0,size=20G"}, "scsi1", true},
		{"legacy bootdisk", map[string]interface{}{
			"bootdisk": "sata1", "scsi0": "a:vm-1-disk-0", "sata1": "a:vm-1-disk-1"}, "sata1", true},
		{"scsi preferred, numerically first", map[string]interface{}{
			"virtio0": "a:vm-1-disk-0", "scsi10": "a:vm-1-disk-1", "scsi2": "a:vm-1-disk-2"}, "scsi2", true},
		{"only a cdrom", map[string]interface{}{"ide2": "local:iso/x.iso,media=cdrom"}, "", false},
		{"no disks", map[string]interface{}{"memory": 2048}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Repeat: the old implementation depended on map iteration order
			for i := 0; i < 20; i++ {
				got, ok := primaryDisk(tt.config)
				if got != tt.want || ok != tt.ok {
					t.Fatalf("primaryDisk = %q, %v; want %q, %v", got, ok, tt.want, tt.ok)
				}
			}
		})
	}
}
