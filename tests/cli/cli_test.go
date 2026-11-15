package cli

import (
	"strings"
	"testing"

	"github.com/scotty-c/prox/internal/fakepve"
)

const upid = fakepve.UPID

// Output formats

func TestVMListTable(t *testing.T) {
	r := run(t, fakepve.New(t), "", "vm", "list")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	golden(t, "vm_list_table", r.stdout)
}

func TestVMListJSON(t *testing.T) {
	r := run(t, fakepve.New(t), "", "vm", "list", "-o", "json")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	golden(t, "vm_list_json", r.stdout)
}

func TestContainerListTable(t *testing.T) {
	r := run(t, fakepve.New(t), "", "ct", "list")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	golden(t, "ct_list_table", r.stdout)
}

func TestStructuredOutputKeepsStdoutClean(t *testing.T) {
	r := run(t, fakepve.New(t), "", "vm", "list", "-o", "json")
	if !strings.HasPrefix(r.stdout, "[") {
		t.Errorf("stdout should be only JSON, got:\n%.200s", r.stdout)
	}
}

// Task logs

func TestLogsShowsTheWholeLog(t *testing.T) {
	r := run(t, fakepve.New(t), "", "logs", upid)
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	if got := strings.Count(r.stdout, "\n"); got != fakepve.LogLines {
		t.Errorf("got %d lines, want %d", got, fakepve.LogLines)
	}
	contains(t, "logs", r.stdout, "log line 120")
}

func TestLogsTailShowsTheLastLines(t *testing.T) {
	r := run(t, fakepve.New(t), "", "logs", upid, "--tail", "3")
	if want := "log line 118\nlog line 119\nlog line 120\n"; r.stdout != want {
		t.Errorf("got %q, want %q", r.stdout, want)
	}
}

func TestLogsFollowReachesTheEnd(t *testing.T) {
	r := run(t, fakepve.New(t), "", "logs", upid, "--follow")
	contains(t, "logs --follow", r.stdout, "log line 120")
}

// Finding VMs and containers

func TestDuplicateNameIsAmbiguous(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "vm", "start", "web")
	if r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "more than one VM is named 'web'", "ID 100 on node pve1", "ID 102 on node pve2")
	if n := srv.Count("POST", "/nodes"); n != 0 {
		t.Errorf("nothing should be started, got %d POSTs", n)
	}
}

func TestNodeFlagDisambiguatesDescribe(t *testing.T) {
	r := run(t, fakepve.New(t), "", "vm", "describe", "web", "--node", "pve1")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	contains(t, "describe", r.stdout, "ID: 100", "Node: pve1")
}

func TestDescribeShowsEveryDiskAndStringConfigValues(t *testing.T) {
	r := run(t, fakepve.New(t), "", "vm", "describe", "100")
	contains(t, "describe", r.stdout, "SCSI0:", "SCSI5:", "QEMU Agent: Enabled", "Memory: 2048 MB")
}

func TestDidYouMeanListsEachNameOnce(t *testing.T) {
	r := run(t, fakepve.New(t), "", "vm", "start", "we")
	if r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	if n := strings.Count(r.stderr, "• web"); n != 1 {
		t.Errorf("web suggested %d times, want 1:\n%s", n, r.stderr)
	}
}

// Confirmation prompts

func TestDeleteWithoutAnswerFailsAndDeletesNothing(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "vm", "delete", "100")
	if r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "Delete VM 'web' (ID 100, running, on pve1)", "pass --force")
	if n := srv.Count("DELETE", "/"); n != 0 {
		t.Errorf("%d DELETEs sent", n)
	}
}

func TestDeleteAnsweredNoCancelsWithExitZero(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "n\n", "vm", "delete", "100")
	if r.code != 0 || srv.Count("DELETE", "/") != 0 {
		t.Errorf("exit %d, DELETEs %d; want 0 and 0", r.code, srv.Count("DELETE", "/"))
	}
	contains(t, "stderr", r.stderr, "Deletion cancelled")
}

func TestDeleteAnsweredYesDeletes(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "y\n", "vm", "delete", "100")
	if r.code != 0 || srv.Count("DELETE", "/nodes/pve1/qemu/100") != 1 {
		t.Errorf("exit %d, DELETEs %d; want 0 and 1\n%s", r.code, srv.Count("DELETE", "/"), r.stderr)
	}
}

func TestDeleteForceSkipsThePrompt(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "ct", "delete", "ct1", "--force")
	if r.code != 0 || srv.Count("DELETE", "/nodes/pve1/lxc/200") != 1 {
		t.Errorf("exit %d, DELETEs %d\n%s", r.code, srv.Count("DELETE", "/"), r.stderr)
	}
}

func TestDeleteUnknownNameFailsBeforeAsking(t *testing.T) {
	r := run(t, fakepve.New(t), "", "vm", "delete", "nope")
	if r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "VM 'nope' not found")
	excludes(t, "stderr", r.stderr, "[y/N]")
}

func TestBulkWithoutAnswerFails(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "vm", "start", "--tag", "prod")
	if r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "web (ID: 100)", "db (ID: 101)", "pass --yes")
}

// Commands and flags

func TestUnknownSubcommandFails(t *testing.T) {
	for _, args := range [][]string{{"vm", "bogus"}, {"node", "bogus"}, {"config", "bogus"}, {"ct", "bogus"}, {"vm", "snapshot", "bogus"}} {
		r := run(t, fakepve.New(t), "", args...)
		if r.code == 0 {
			t.Errorf("prox %v exited 0", args)
		}
		contains(t, "stderr", r.stderr, "unknown command")
	}
}

func TestUnknownSubcommandSuggestsCloseMatch(t *testing.T) {
	r := run(t, fakepve.New(t), "", "vm", "lst")
	contains(t, "stderr", r.stderr, "Did you mean this?", "list")
}

func TestNodeListAlias(t *testing.T) {
	r := run(t, fakepve.New(t), "", "node", "list")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	contains(t, "node list", r.stdout, "pve1", "pve2")
}

func TestVersionFlag(t *testing.T) {
	r := run(t, fakepve.New(t), "", "--version")
	if r.code != 0 || !strings.HasPrefix(r.stdout, "prox ") {
		t.Errorf("exit %d, stdout %q", r.code, r.stdout)
	}
}

func TestStatusQuietIsQuiet(t *testing.T) {
	r := run(t, fakepve.New(t), "", "status", "-q")
	excludes(t, "stdout", r.stdout, "Connecting")
}

// Disk resize

func TestEditDiskWaitsForTheResize(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "vm", "edit", "100", "--disk", "5")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	if srv.Count("PUT", "/nodes/pve1/qemu/100/resize") != 1 || srv.Count("GET", "/nodes/pve1/tasks/") == 0 {
		t.Errorf("expected one resize and a task poll, got %v", srv.Requests())
	}
	contains(t, "stdout", r.stdout, "disk scsi0 grown by 5 GB")
}

// Errors from Proxmox

func TestProxmoxErrorReasonIsShown(t *testing.T) {
	srv := fakepve.New(t)
	srv.FailRequest("POST /nodes/pve1/qemu/101/status/start", "VM 101 is locked (backup)")
	r := run(t, srv, "", "vm", "start", "db")
	if r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "VM 101 is locked (backup)")
}
