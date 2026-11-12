package client

import (
	"encoding/json"
	"testing"
)

func TestSumGuestFilesystemUsage(t *testing.T) {
	// Trimmed real-world get-fsinfo output: root, a bind mount of the same
	// filesystem, /boot/efi, and a tmpfs with no backing disk.
	raw := `{"data":{"result":[
		{"name":"sda1","mountpoint":"/","type":"ext4","used-bytes":5000000000,"total-bytes":20000000000,"disk":[{"dev":"/dev/sda1"}]},
		{"name":"sda1","mountpoint":"/var/lib/docker","type":"ext4","used-bytes":5000000000,"total-bytes":20000000000,"disk":[{"dev":"/dev/sda1"}]},
		{"name":"sda15","mountpoint":"/boot/efi","type":"vfat","used-bytes":6000000,"total-bytes":100000000,"disk":[{"dev":"/dev/sda15"}]},
		{"name":"tmpfs","mountpoint":"/run","type":"tmpfs","used-bytes":1000,"total-bytes":1000000,"disk":[]}
	]}}`
	var resp struct {
		Data struct {
			Result []guestFilesystem `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	used, ok := sumGuestFilesystemUsage(resp.Data.Result)
	if !ok || used != 5006000000 {
		t.Fatalf("used = %d, %v; want 5006000000, true", used, ok)
	}

	if _, ok := sumGuestFilesystemUsage(nil); ok {
		t.Fatal("expected no usage for empty result")
	}
}
