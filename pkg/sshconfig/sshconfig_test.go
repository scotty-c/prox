package sshconfig

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const sample = `Include ~/.ssh/config.d/*

Host web
    HostName 10.0.0.5
    User admin

# Work machines
Match host *.corp exec "test -f ~/.corp"
    ProxyJump bastion

Host web db
    User shared

Host db
    HostName=10.0.0.6
    Port 2222
	IdentityFile ~/.ssh/db_key

Host *
    ServerAliveInterval 30
`

func TestRemoveHostKeepsFollowingMatchBlockAndComments(t *testing.T) {
	got, removed, ok := RemoveHost(sample, "web", true)
	if !ok {
		t.Fatal("expected web to be removed")
	}
	if strings.Contains(removed, "Match") || strings.Contains(removed, "# Work machines") {
		t.Fatalf("removed text included the following block or its comment:\n%s", removed)
	}
	for _, keep := range []string{"Include ~/.ssh/config.d/*", "# Work machines", "Match host *.corp", "ProxyJump bastion", "Host web db", "Host *"} {
		if !strings.Contains(got, keep) {
			t.Errorf("result lost %q:\n%s", keep, got)
		}
	}
	if strings.Contains(got, "10.0.0.5") {
		t.Errorf("result still contains the removed block:\n%s", got)
	}
}

func TestRemoveHostExactLeavesSharedBlocks(t *testing.T) {
	got, _, _ := RemoveHost(sample, "db", true)
	if !strings.Contains(got, "Host web db") {
		t.Fatal("exact removal should not touch a block shared with another alias")
	}
	if strings.Contains(got, "10.0.0.6") {
		t.Fatal("exact removal should remove the db block")
	}
}

func TestRemoveHostNonExactRemovesSharedBlocks(t *testing.T) {
	got, _, _ := RemoveHost(sample, "db", false)
	if strings.Contains(got, "Host web db") || strings.Contains(got, "10.0.0.6") {
		t.Fatalf("non-exact removal should remove every block naming db:\n%s", got)
	}
	if !strings.Contains(got, "Host web\n") {
		t.Fatal("non-exact removal removed an unrelated block")
	}
}

func TestRemoveHostMissing(t *testing.T) {
	got, removed, ok := RemoveHost(sample, "nope", false)
	if ok || removed != "" || got != sample {
		t.Fatalf("expected no change, got ok=%v removed=%q", ok, removed)
	}
}

func TestAddHostIsIdempotent(t *testing.T) {
	entry := "Host vm1\n    HostName 10.0.0.9\n    User me\n"
	once := AddHost(sample, "vm1", entry)
	twice := AddHost(once, "vm1", entry)
	if once != twice {
		t.Fatalf("adding the same entry twice changed the file:\n--- once\n%s\n--- twice\n%s", once, twice)
	}
	// Removing it again restores the original file
	if restored, _, _ := RemoveHost(once, "vm1", true); restored != sample {
		t.Fatalf("remove after add did not restore the file:\n%s", restored)
	}
}

func TestAddHostKeepsGlobalSectionGlobal(t *testing.T) {
	content := "Include ~/.orbstack/ssh/config\nServerAliveInterval 30\n\n# Git hosting\nHost github.com\n    User git\n"
	entry := "Host vm1\n    HostName 10.0.0.9\n"
	want := "Include ~/.orbstack/ssh/config\nServerAliveInterval 30\n\n" + entry + "\n# Git hosting\nHost github.com\n    User git\n"
	if got := AddHost(content, "vm1", entry); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestAddHostNoBlocks(t *testing.T) {
	entry := "Host vm1\n    HostName 10.0.0.9\n"
	if got := AddHost("", "vm1", entry); got != entry {
		t.Fatalf("empty file: got %q", got)
	}
	if got := AddHost("ServerAliveInterval 30\n", "vm1", entry); got != "ServerAliveInterval 30\n\n"+entry {
		t.Fatalf("globals only: got %q", got)
	}
}

func TestAddHostRepairsEntriesWrittenAtTop(t *testing.T) {
	// Older prox versions put entries in front of the global section
	old := "Host vm1\n    HostName 10.0.0.1\n    User root\n\nInclude ~/.orbstack/ssh/config\n\nHost github.com\n    User git\n"
	entry := "Host vm1\n    HostName 10.0.0.2\n"
	want := "Include ~/.orbstack/ssh/config\n\n" + entry + "\nHost github.com\n    User git\n"
	if got := AddHost(old, "vm1", entry); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveHostUnindentedBlock(t *testing.T) {
	content := "Host a\nHostName 1.1.1.1\nUser x\n# about b\nHost b\nHostName 2.2.2.2\n"
	got, _, _ := RemoveHost(content, "a", true)
	if want := "# about b\nHost b\nHostName 2.2.2.2\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestParseEntries(t *testing.T) {
	got := ParseEntries(sample)
	want := []Entry{
		{Hosts: []string{"web"}, HostName: "10.0.0.5", User: "admin", Port: "22"},
		{Hosts: []string{"web", "db"}, User: "shared", Port: "22"},
		{Hosts: []string{"db"}, HostName: "10.0.0.6", Port: "2222", Identity: "~/.ssh/db_key"},
		{Hosts: []string{"*"}, Port: "22"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseEntries mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

func TestWriteFileBacksUpAndFollowsSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "dotfiles-config")
	if err := os.WriteFile(real, []byte("old\n"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "config")
	if err := os.Symlink(real, link); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	if err := WriteFile(link, []byte("new\n")); err != nil {
		t.Fatal(err)
	}

	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("symlink was replaced by a regular file")
	}
	if data, _ := os.ReadFile(real); string(data) != "new\n" {
		t.Fatalf("target content = %q", data)
	}
	if data, _ := os.ReadFile(real + ".prox.bak"); string(data) != "old\n" {
		t.Fatalf("backup content = %q", data)
	}
	if fi, _ := os.Stat(real); fi.Mode().Perm() != 0644 {
		t.Fatalf("permissions changed to %v", fi.Mode().Perm())
	}
}

func TestWriteFileCreatesNew(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := WriteFile(path, []byte("x\n")); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0600 {
		t.Fatalf("new file permissions = %v, want 0600", fi.Mode().Perm())
	}
	if _, err := os.Stat(path + ".prox.bak"); !os.IsNotExist(err) {
		t.Fatal("no backup should be made when there was no file")
	}
}

func TestParseEntriesRecognisesTheEntriesProxWrote(t *testing.T) {
	content := `Host mine
    HostName 10.0.0.1

Host web
    ` + ManagedMarker + `
    HostName 10.0.0.2
    User scott

# a comment about the next block
Host db other
    ` + ManagedMarker + `
    HostName 10.0.0.3
`
	entries := ParseEntries(content)
	if len(entries) != 3 {
		t.Fatalf("entries = %+v", entries)
	}
	if entries[0].Managed || !entries[1].Managed || !entries[2].Managed {
		t.Errorf("Managed = %v %v %v, want false true true", entries[0].Managed, entries[1].Managed, entries[2].Managed)
	}
	if entries[1].HostName != "10.0.0.2" || entries[1].User != "scott" {
		t.Errorf("directives after the marker are still read: %+v", entries[1])
	}
}

func TestStaleManagedOnlyReturnsSingleHostEntriesProxWrote(t *testing.T) {
	entries := []Entry{
		{Hosts: []string{"gone-by-hand"}},             // not written by prox
		{Hosts: []string{"gone"}, Managed: true},      // stale
		{Hosts: []string{"here"}, Managed: true},      // still exists
		{Hosts: []string{"a", "b"}, Managed: true},    // several hosts: never touched
		{Hosts: []string{"also-gone"}, Managed: true}, // stale
	}
	exists := func(alias string) bool { return alias == "here" }
	got := StaleManaged(entries, exists)
	if strings.Join(got, ",") != "gone,also-gone" {
		t.Errorf("StaleManaged = %v, want gone,also-gone", got)
	}
}

func TestMarkerTravelsWithTheBlock(t *testing.T) {
	block := "Host web\n    " + ManagedMarker + "\n    HostName 10.0.0.2\n"
	content := "Host keep\n    HostName 10.0.0.1\n\n" + block + "\n# trailing note\nHost last\n    HostName 10.0.0.9\n"

	// Removing the entry takes its marker with it and leaves the neighbours alone
	removed, taken, found := RemoveHost(content, "web", true)
	if !found || strings.Contains(removed, ManagedMarker) || !strings.Contains(removed, "Host keep") ||
		!strings.Contains(removed, "# trailing note") || !strings.Contains(removed, "Host last") {
		t.Errorf("RemoveHost left %q (found %v)", removed, found)
	}
	if !strings.Contains(taken, ManagedMarker) {
		t.Errorf("the removed text should include the marker: %q", taken)
	}

	// Replacing it does not pile up markers
	replaced := AddHost(content, "web", "Host web\n    "+ManagedMarker+"\n    HostName 10.0.0.77\n")
	if n := strings.Count(replaced, ManagedMarker); n != 1 || !strings.Contains(replaced, "10.0.0.77") || strings.Contains(replaced, "10.0.0.2") {
		t.Errorf("AddHost result (markers: %d):\n%s", n, replaced)
	}
}
