package vm

import "testing"

func TestFormatDiskUsage(t *testing.T) {
	const gib = 1024 * 1024 * 1024
	tests := []struct {
		used, max uint64
		want      string
	}{
		{0, 0, "N/A"},
		{0, 32 * gib, "N/A/32.0 GB"},
		{8 * gib, 32 * gib, "8.0 GB/32.0 GB (25.0%)"},
	}
	for _, tt := range tests {
		if got := formatDiskUsage(tt.used, tt.max); got != tt.want {
			t.Errorf("formatDiskUsage(%d, %d) = %q, want %q", tt.used, tt.max, got, tt.want)
		}
	}
}
