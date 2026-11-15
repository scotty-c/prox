package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"
	"time"

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
	contains(t, "stderr", r.stderr, "Delete VM 'web' (ID 100, running, on pve1)", "pass --yes")
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
	if n := strings.Count(r.stderr, "VM 101 is locked"); n != 1 {
		t.Errorf("the error was printed %d times, want once:\n%s", n, r.stderr)
	}
}

func TestEachFailureIsPrintedOnce(t *testing.T) {
	srv := fakepve.New(t)
	srv.FailRequest("DELETE /nodes/pve1/lxc/200", "container is locked (snapshot)")
	for _, args := range [][]string{
		{"vm", "describe", "nope"},
		{"ct", "start", "nope"},
		{"ct", "delete", "ct1", "--force"},
		{"vm", "edit", "100"},
		{"node", "describe", "zzz"},
		{"ssh", "nope", "--print"},
	} {
		r := run(t, srv, "", args...)
		if r.code != 1 {
			t.Errorf("prox %v: exit %d, want 1", args, r.code)
		}
		if n := strings.Count(r.stderr, "Error:"); n != 1 {
			t.Errorf("prox %v printed %d error lines, want 1:\n%s", args, n, r.stderr)
		}
	}
}

// Error reporting: each failure is printed once, with a tip that fits it

func TestUnreachableServerSaysSoWithATip(t *testing.T) {
	srv := fakepve.New(t)
	srv.Close() // nothing is listening any more
	for _, args := range [][]string{{"vm", "list"}, {"vm", "start", "web"}, {"ct", "list"}, {"status"}, {"node", "ls"}} {
		r := run(t, srv, "", args...)
		if r.code != 1 {
			t.Errorf("prox %v: exit %d, want 1", args, r.code)
		}
		contains(t, "prox "+strings.Join(args, " ")+" stderr", r.stderr, "cannot reach the Proxmox server", "Tip: Check the URL")
		excludes(t, "stderr", r.stderr, "authentication failed")
		if n := strings.Count(r.stderr, "cannot reach the Proxmox server"); n != 1 {
			t.Errorf("prox %v printed the error %d times:\n%s", args, n, r.stderr)
		}
	}
}

func TestRejectedLoginIsAnAuthenticationError(t *testing.T) {
	srv := fakepve.New(t)
	srv.FailRequestWithStatus("POST /access/ticket", 401, "authentication failure")
	r := run(t, srv, "", "vm", "list")
	if r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "authentication failed", "Tip: Check your credentials")
	excludes(t, "stderr", r.stderr, "cannot reach")
}

func TestMissingPrivilegeGetsAPermissionTip(t *testing.T) {
	srv := fakepve.New(t)
	srv.FailRequestWithStatus("POST /nodes/pve1/qemu/100/status/start", 403, "Permission check failed (/vms/100, VM.PowerMgmt)")
	r := run(t, srv, "", "vm", "start", "100")
	contains(t, "stderr", r.stderr, "VM.PowerMgmt", "Tip: Your Proxmox user lacks the privileges")
}

func TestNoSavedConfigSaysHowToSetOneUp(t *testing.T) {
	cmd := exec.Command(binary, "vm", "list")
	cmd.Env = []string{"HOME=" + t.TempDir(), "PATH=" + os.Getenv("PATH")}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		t.Fatal("expected a failure")
	}
	contains(t, "stderr", stderr.String(), "no configuration found", "Tip: Run 'prox config setup'")
}

func TestBadFlagValueIsReportedOnce(t *testing.T) {
	r := run(t, fakepve.New(t), "", "vm", "list", "-o", "bogus")
	if r.code != 1 || strings.Count(r.stderr, `invalid --output "bogus"`) != 1 {
		t.Errorf("exit %d, stderr:\n%s", r.code, r.stderr)
	}
}

// Output streams and wording

func TestListsAreSortedByID(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "vm", "list", "-o", "json")
	var vms []struct{ ID int }
	if err := json.Unmarshal([]byte(r.stdout), &vms); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, r.stdout)
	}
	var ids []int
	for _, v := range vms {
		ids = append(ids, v.ID)
	}
	if !sort.IntsAreSorted(ids) || len(ids) != 4 {
		t.Errorf("VM IDs = %v, want 4 in ascending order", ids)
	}

	r = run(t, srv, "", "node", "ls", "-o", "json")
	var nodes []struct{ Node string }
	_ = json.Unmarshal([]byte(r.stdout), &nodes)
	if len(nodes) != 2 || nodes[0].Node != "pve1" {
		t.Errorf("nodes = %v", nodes)
	}
}

func TestProgressGoesToStderrAndStdoutHoldsTheResult(t *testing.T) {
	r := run(t, fakepve.New(t), "", "vm", "list")
	contains(t, "stderr", r.stderr, "Retrieving virtual machines")
	excludes(t, "stdout", r.stdout, "Retrieving")
	if !strings.HasPrefix(r.stdout, "╭") {
		t.Errorf("stdout should start with the table, got:\n%.80s", r.stdout)
	}
}

func TestStartNamesTheVMAndReportsOneResultLine(t *testing.T) {
	r := run(t, fakepve.New(t), "", "vm", "start", "100", "--wait")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	contains(t, "stderr", r.stderr, "Starting VM web (ID 100) on node pve1")
	contains(t, "stdout", r.stdout, "Task started: "+upid, "VM web (ID 100) started in ")
	excludes(t, "stdout", r.stdout, "command issued successfully", "Task ID:")
}

func TestStartWithoutWaitPointsAtTheLog(t *testing.T) {
	r := run(t, fakepve.New(t), "", "vm", "start", "100")
	contains(t, "stdout", r.stdout, "Task started: "+upid)
	contains(t, "stderr", r.stderr, "prox logs --follow "+upid)
	excludes(t, "stdout", r.stdout, "started in")
}

func TestDeleteAndShutdownNameTheVM(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "vm", "delete", "100", "--force", "--wait")
	contains(t, "delete stdout", r.stdout, "VM web (ID 100) deleted")
	r = run(t, srv, "", "vm", "shutdown", "100", "--wait")
	contains(t, "shutdown stdout", r.stdout, "VM web (ID 100) shut down in ")
}

// prox ssh

func TestSSHAmbiguousNameStopsInsteadOfTryingContainers(t *testing.T) {
	r := run(t, fakepve.New(t), "", "ssh", "web", "--print")
	if r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "more than one VM is named 'web'", "ID 100 on node pve1")
	excludes(t, "stderr", r.stderr, "not found", "Container error")
	if n := strings.Count(r.stderr, "Error:"); n != 1 {
		t.Errorf("%d error lines, want 1:\n%s", n, r.stderr)
	}
}

func TestSSHNotFoundSuggestsFromVMsAndContainers(t *testing.T) {
	r := run(t, fakepve.New(t), "", "ssh", "ct3", "--print")
	contains(t, "stderr", r.stderr, "'ct3' not found as a VM or container", "Did you mean one of these?", "• ct1", "prox ct list")
	excludes(t, "stderr", r.stderr, "VM error", "Container error")
}

func TestSSHPrintsTheCommandForAContainer(t *testing.T) {
	r := run(t, fakepve.New(t), "", "ssh", "ct1", "--print", "--user", "root")
	if r.code != 0 || strings.TrimSpace(r.stdout) != "ssh root@10.0.0.50" {
		t.Errorf("exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}
}

func TestSSHExplainsAVMWithoutAnAddress(t *testing.T) {
	r := run(t, fakepve.New(t), "", "ssh", "100", "--print")
	if r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "no IP address found for VM 'web'", "Make sure the resource is running")
}

func TestSSHDeleteDoesNotNeedTheNetworkWhenTheHostIsInTheConfig(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(home+"/.ssh", 0o700); err != nil {
		t.Fatal(err)
	}
	config := "Host web\n    HostName 10.0.0.9\n\nHost keep\n    HostName 10.0.0.1\n"
	if err := os.WriteFile(home+"/.ssh/config", []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := fakepve.New(t)
	r := runInHome(t, home, srv, "", "ssh", "web", "--delete")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	if n := len(srv.Requests()); n != 0 {
		t.Errorf("made %d requests to Proxmox, want none: %v", n, srv.Requests())
	}
	got, _ := os.ReadFile(home + "/.ssh/config")
	excludes(t, "ssh config", string(got), "Host web")
	contains(t, "ssh config", string(got), "Host keep")
}

// describe output

func TestContainerDescribeIsOrderedAndStableAcrossRuns(t *testing.T) {
	srv := fakepve.New(t)
	first := run(t, srv, "", "ct", "describe", "ct1")
	if first.code != 0 {
		t.Fatalf("exit %d: %s", first.code, first.stderr)
	}
	for i := 0; i < 5; i++ {
		if again := run(t, srv, "", "ct", "describe", "ct1"); again.stdout != first.stdout {
			t.Fatalf("describe output changed between runs:\n--- first ---\n%s\n--- again ---\n%s", first.stdout, again.stdout)
		}
	}

	out := first.stdout
	for _, pair := range [][2]string{
		{"Network 0:", "Network 1:"}, {"Network 1:", "Network 2:"},
		{"Mount Point 0:", "Mount Point 1:"}, {"Mount Point 1:", "Mount Point 10:"},
	} {
		a, b := strings.Index(out, pair[0]), strings.Index(out, pair[1])
		if a < 0 || b < 0 || a > b {
			t.Errorf("%q should come before %q in:\n%s", pair[0], pair[1], out)
		}
	}
}

func TestContainerDescribeLabelsTheContainersState(t *testing.T) {
	r := run(t, fakepve.New(t), "", "ct", "describe", "ct1")
	contains(t, "describe", r.stdout, "State: running")
	excludes(t, "describe", r.stdout, "VM Status")
}

func TestNodeDescribeShowsEachFigureOnce(t *testing.T) {
	r := run(t, fakepve.New(t), "", "node", "describe", "pve1")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	for _, label := range []string{"CPU:", "Memory:", "Uptime:"} {
		if n := strings.Count(r.stdout, label); n != 1 {
			t.Errorf("%q appears %d times, want once:\n%s", label, n, r.stdout)
		}
	}
	excludes(t, "describe", r.stdout, "Runtime Status")
}

// config profiles

func profiles(t *testing.T, home string, srv *fakepve.Server) []string {
	t.Helper()
	r := runInHome(t, home, srv, "", "config", "list", "-o", "json")
	var list []struct{ Name string }
	if err := json.Unmarshal([]byte(r.stdout), &list); err != nil {
		t.Fatalf("config list: %v\n%s%s", err, r.stdout, r.stderr)
	}
	var names []string
	for _, p := range list {
		names = append(names, p.Name)
	}
	return names
}

func TestConfigCreateRefusesToSaveAnIncompleteProfile(t *testing.T) {
	home, srv := t.TempDir(), fakepve.New(t)
	r := runInHome(t, home, srv, "", "config", "create", "lab", "-u", "root@pam")
	if r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "missing --password, --url")
	if got := profiles(t, home, srv); len(got) != 0 {
		t.Errorf("a profile was saved anyway: %v", got)
	}
	// no flags at all
	if r := runInHome(t, home, srv, "", "config", "create", "lab"); r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	if got := profiles(t, home, srv); len(got) != 0 {
		t.Errorf("a profile was saved anyway: %v", got)
	}
}

func TestConfigCreateSavesAProfileThatHasEverything(t *testing.T) {
	home, srv := t.TempDir(), fakepve.New(t)
	r := runInHome(t, home, srv, "", "config", "create", "lab", "-u", "root@pam", "-p", "pw", "-l", "https://pve:8006")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	if got := profiles(t, home, srv); len(got) != 1 || got[0] != "lab" {
		t.Errorf("profiles = %v", got)
	}
}

func TestConfigDeleteAsksFirstAndNeedsAnAnswer(t *testing.T) {
	home, srv := t.TempDir(), fakepve.New(t)
	runInHome(t, home, srv, "", "config", "create", "lab", "-u", "u@pam", "-p", "pw", "-l", "https://pve:8006")

	r := runInHome(t, home, srv, "", "config", "delete", "lab")
	if r.code != 1 {
		t.Errorf("no answer: exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "Delete profile 'lab'", "pass --yes")

	if r := runInHome(t, home, srv, "n\n", "config", "delete", "lab"); r.code != 0 {
		t.Errorf("answered no: exit %d, want 0", r.code)
	}
	if got := profiles(t, home, srv); len(got) != 1 {
		t.Fatalf("profile should still exist after 'no': %v", got)
	}

	if r := runInHome(t, home, srv, "y\n", "config", "delete", "lab"); r.code != 0 {
		t.Fatalf("answered yes: exit %d: %s", r.code, r.stderr)
	}
	if got := profiles(t, home, srv); len(got) != 0 {
		t.Errorf("profile should be gone: %v", got)
	}
}

func TestConfigDeleteCanRemoveTheDefaultProfile(t *testing.T) {
	home, srv := t.TempDir(), fakepve.New(t)
	runInHome(t, home, srv, "", "config", "create", "default", "-u", "u@pam", "-p", "pw", "-l", "https://pve:8006")
	runInHome(t, home, srv, "", "config", "create", "other", "-u", "u@pam", "-p", "pw", "-l", "https://pve:8006")

	r := runInHome(t, home, srv, "", "config", "delete", "default", "--force")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	if got := profiles(t, home, srv); len(got) != 1 || got[0] != "other" {
		t.Errorf("profiles = %v, want just 'other'", got)
	}
}

func TestConfigDeleteUnknownProfileFailsBeforeAsking(t *testing.T) {
	r := run(t, fakepve.New(t), "", "config", "delete", "nope")
	if r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "profile 'nope' does not exist")
	excludes(t, "stderr", r.stderr, "[y/N]")
}

// -y/--yes, clone IDs and ct create options

func TestYesAndForceBothSkipTheConfirmation(t *testing.T) {
	for _, flag := range []string{"-y", "--yes", "-f", "--force"} {
		srv := fakepve.New(t)
		r := run(t, srv, "", "vm", "delete", "100", flag)
		if r.code != 0 || srv.Count("DELETE", "/nodes/pve1/qemu/100") != 1 {
			t.Errorf("vm delete %s: exit %d, DELETEs %d\n%s", flag, r.code, srv.Count("DELETE", "/"), r.stderr)
		}
		srv = fakepve.New(t)
		r = run(t, srv, "", "ct", "delete", "ct1", flag)
		if r.code != 0 || srv.Count("DELETE", "/nodes/pve1/lxc/200") != 1 {
			t.Errorf("ct delete %s: exit %d, DELETEs %d\n%s", flag, r.code, srv.Count("DELETE", "/"), r.stderr)
		}
	}
}

func TestYesSkipsSnapshotAndConfigPrompts(t *testing.T) {
	srv := fakepve.New(t)
	if r := run(t, srv, "", "vm", "snapshot", "delete", "100", "snap1", "--yes"); r.code != 0 || srv.Count("DELETE", "/nodes/pve1/qemu/100/snapshot/snap1") != 1 {
		t.Errorf("snapshot delete: exit %d\n%s", r.code, r.stderr)
	}
	if r := run(t, srv, "", "vm", "snapshot", "rollback", "100", "snap1", "-y"); r.code != 0 || srv.Count("POST", "/nodes/pve1/qemu/100/snapshot/snap1/rollback") != 1 {
		t.Errorf("snapshot rollback: exit %d\n%s", r.code, r.stderr)
	}
	home := t.TempDir()
	runInHome(t, home, srv, "", "config", "create", "lab", "-u", "u@pam", "-p", "pw", "-l", "https://pve:8006")
	if r := runInHome(t, home, srv, "", "config", "delete", "lab", "-y"); r.code != 0 {
		t.Errorf("config delete -y: exit %d\n%s", r.code, r.stderr)
	}
}

func TestVMCloneWithoutAnIDUsesTheNextFreeOne(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "vm", "clone", "100", "web2", "--wait")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	body := srv.Body("POST /nodes/pve1/qemu/100/clone")
	contains(t, "clone request", body, `"newid":103`, `"name":"web2"`)
	contains(t, "stderr", r.stderr, "next free VM ID: 103")
}

func TestVMCloneStillTakesAnExplicitID(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "vm", "clone", "100", "300", "web2")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	contains(t, "clone request", srv.Body("POST /nodes/pve1/qemu/100/clone"), `"newid":300`, `"name":"web2"`)
}

func TestVMCloneNeedsAName(t *testing.T) {
	r := run(t, fakepve.New(t), "", "vm", "clone", "100", "300")
	if r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "a name for the new VM is required")
}

func TestContainerCloneWithoutAnIDUsesTheNextFreeOne(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "ct", "clone", "ct1", "ct1-copy", "--full")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	contains(t, "clone request", srv.Body("POST /nodes/pve1/lxc/200/clone"), `"newid":103`, `"hostname":"ct1-copy"`)
}

func TestContainerCreateAcceptsCPUAndIDAndCanStart(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "ct", "create", "web2", "debian:12", "--cpu", "2", "--id", "150", "--start")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	contains(t, "create request", srv.Body("POST /nodes/pve1/lxc"), `"cores":2`, `"vmid":150`, `"hostname":"web2"`)
	if srv.Count("POST", "/nodes/pve1/lxc/150/status/start") != 1 {
		t.Errorf("container should have been started: %v", srv.Requests())
	}
	contains(t, "stdout", r.stdout, "Container web2 (ID: 150) created on node pve1")
}

func TestContainerCreateWithoutStartLeavesItStopped(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "ct", "create", "web2", "debian:12")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	if n := srv.Count("POST", "/nodes/pve1/lxc/"); n != 0 {
		t.Errorf("nothing should be started, got %d POSTs", n)
	}
	contains(t, "stderr", r.stderr, "prox ct start web2")
}

func TestContainerCreateRejectsConflictingAliasesBeforeDoingAnything(t *testing.T) {
	for _, args := range [][]string{
		{"ct", "create", "web2", "debian:12", "--cpu", "2", "--cores", "3"},
		{"ct", "create", "web2", "debian:12", "--id", "150", "--vmid", "151"},
		{"ct", "create", "web2", "debian:12", "--start", "--no-wait"},
	} {
		srv := fakepve.New(t)
		r := run(t, srv, "", args...)
		if r.code != 1 {
			t.Errorf("prox %v: exit %d, want 1", args, r.code)
		}
		if srv.Count("POST", "/") != 0 {
			t.Errorf("prox %v made changes: %v", args, srv.Requests())
		}
	}
}

func TestVMTestCommandIsHidden(t *testing.T) {
	r := run(t, fakepve.New(t), "", "vm", "--help")
	excludes(t, "vm help", r.stdout, "Test connection to Proxmox server")
	if r := run(t, fakepve.New(t), "", "vm", "test"); r.code != 0 {
		t.Errorf("hidden command should still work: exit %d\n%s", r.code, r.stderr)
	}
}

// Ctrl-C

func TestCtrlCStopsARequestThatIsStuck(t *testing.T) {
	srv := fakepve.New(t)
	srv.Hang("GET /cluster/resources")
	r, took := startInterrupted(t, srv, "GET /cluster/resources", "vm", "list")
	if r.code != 130 {
		t.Errorf("exit %d, want 130 (the usual status after Ctrl-C)\n%s", r.code, r.stderr)
	}
	contains(t, "stderr", r.stderr, "Interrupted")
	excludes(t, "stderr", r.stderr, "Error:", "context canceled")
	if took > 3*time.Second {
		t.Errorf("took %s to stop", took)
	}
}

func TestCtrlCWhileWaitingForATaskSaysTheTaskKeepsRunning(t *testing.T) {
	srv := fakepve.New(t)
	srv.Hang("GET /nodes/pve1/tasks/")
	r, _ := startInterrupted(t, srv, "GET /nodes/pve1/tasks/", "vm", "start", "100", "--wait")
	if r.code == 0 {
		t.Errorf("exit 0 after being interrupted")
	}
	contains(t, "stderr", r.stderr, "stopped waiting (interrupted)", "the task is still running on the server", "prox logs --follow "+upid)
	if n := srv.Count("POST", "/nodes/pve1/qemu/100/status/start"); n != 1 {
		t.Errorf("the start request was sent %d times, want once", n)
	}
}

// Shell completion

// candidates returns the completion candidates cobra printed (one per line,
// before the ":<directive>" line), as name -> description
func candidates(stdout string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(stdout, "\n") {
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		name, desc, _ := strings.Cut(line, "\t")
		out[name] = desc
	}
	return out
}

func TestCompletionOffersOnlyWhatTheCommandCanActOn(t *testing.T) {
	srv := fakepve.New(t)
	tests := []struct {
		args       []string
		want, deny []string
	}{
		// VM 100 "web" and 1000 "big" run; 101 "db" and 102 "web" are stopped
		{[]string{"vm", "start"}, []string{"db", "101", "102"}, []string{"big", "1000", "100"}},
		{[]string{"vm", "shutdown"}, []string{"big", "1000", "100"}, []string{"db", "101", "102"}},
		{[]string{"vm", "reboot"}, []string{"big", "100"}, []string{"db", "102"}},
		{[]string{"vm", "describe"}, []string{"big", "db", "100", "101"}, nil}, // anything
		// container 200 "ct1" runs; 201 "ct2" is stopped
		{[]string{"ct", "start"}, []string{"ct2", "201"}, []string{"ct1", "200"}},
		{[]string{"ct", "stop"}, []string{"ct1", "200"}, []string{"ct2", "201"}},
		{[]string{"ct", "enter"}, []string{"ct1"}, []string{"ct2"}},
	}
	for _, tt := range tests {
		r := run(t, srv, "", append([]string{"__complete"}, append(tt.args, "")...)...)
		got := candidates(r.stdout)
		for _, w := range tt.want {
			if _, ok := got[w]; !ok {
				t.Errorf("prox %v <TAB> should offer %q, got %v", tt.args, w, got)
			}
		}
		for _, d := range tt.deny {
			if _, ok := got[d]; ok {
				t.Errorf("prox %v <TAB> should not offer %q, got %v", tt.args, d, got)
			}
		}
	}
}

func TestCompletionDescribesEachCandidate(t *testing.T) {
	r := run(t, fakepve.New(t), "", "__complete", "vm", "shutdown", "")
	got := candidates(r.stdout)
	if got["big"] != "VM 1000 on pve2, running" {
		t.Errorf("name description = %q", got["big"])
	}
	if got["1000"] != "VM big on pve2, running" {
		t.Errorf("ID description = %q", got["1000"])
	}
	nodes := candidates(run(t, fakepve.New(t), "", "__complete", "vm", "migrate", "web", "").stdout)
	if nodes["pve2"] != "online" {
		t.Errorf("node description = %q (candidates %v)", nodes["pve2"], nodes)
	}
}

func TestCompletionOnlyCompletesTheFirstArgument(t *testing.T) {
	r := run(t, fakepve.New(t), "", "__complete", "vm", "start", "db", "")
	if got := candidates(r.stdout); len(got) != 0 {
		t.Errorf("second argument should offer nothing, got %v", got)
	}
}

func TestCompletionGivesUpWhenTheClusterDoesNotAnswer(t *testing.T) {
	srv := fakepve.New(t)
	srv.Hang("GET /cluster/resources")
	began := time.Now()
	r := run(t, srv, "", "__complete", "vm", "start", "")
	took := time.Since(began)
	if took > 8*time.Second {
		t.Errorf("TAB took %s with an unresponsive cluster, want a few seconds at most", took)
	}
	if got := candidates(r.stdout); len(got) != 0 {
		t.Errorf("nothing to offer, got %v", got)
	}
}

// Structured output

func TestListJSONHasMachineFriendlyFieldsAlongsideTheOldOnes(t *testing.T) {
	srv := fakepve.New(t)
	for _, tc := range []struct {
		args []string
		name string
	}{{[]string{"vm", "list", "-o", "json"}, "web"}, {[]string{"ct", "list", "-o", "json"}, "ct1"}} {
		r := run(t, srv, "", tc.args...)
		var rows []map[string]interface{}
		if err := json.Unmarshal([]byte(r.stdout), &rows); err != nil {
			t.Fatalf("%v: %v\n%s", tc.args, err, r.stdout)
		}
		var row map[string]interface{}
		for _, candidate := range rows {
			if candidate["name"] == tc.name && candidate["status"] == "running" {
				row = candidate
			}
		}
		if row == nil {
			t.Fatalf("%v: no running %s in %v", tc.args, tc.name, rows)
		}
		// the original fields keep their original meaning
		for _, key := range []string{"id", "name", "status", "node", "cpus", "memory", "max_memory", "disk", "max_disk", "uptime", "ip"} {
			if _, ok := row[key]; !ok {
				t.Errorf("%v: original field %q is gone", tc.args, key)
			}
		}
		if _, isString := row["uptime"].(string); !isString {
			t.Errorf("%v: uptime should still be the human string, got %#v", tc.args, row["uptime"])
		}
		// the new ones
		if cpu, ok := row["cpu_percent"].(float64); !ok || cpu <= 0 || cpu != row["cpus"].(float64) && int(cpu) != int(row["cpus"].(float64)) {
			t.Errorf("%v: cpu_percent = %#v (cpus %v)", tc.args, row["cpu_percent"], row["cpus"])
		}
		if n, ok := row["cpu_count"].(float64); !ok || n < 1 {
			t.Errorf("%v: cpu_count = %#v", tc.args, row["cpu_count"])
		}
		if secs, ok := row["uptime_seconds"].(float64); !ok || secs <= 0 {
			t.Errorf("%v: uptime_seconds = %#v", tc.args, row["uptime_seconds"])
		}
		if _, ok := row["tag_list"].([]interface{}); !ok {
			t.Errorf("%v: tag_list should be an array even when empty, got %#v", tc.args, row["tag_list"])
		}
	}
}

func TestTagListIsAnArrayOfTags(t *testing.T) {
	r := run(t, fakepve.New(t), "", "vm", "list", "-o", "json")
	var rows []struct {
		ID      int      `json:"id"`
		TagList []string `json:"tag_list"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		switch row.ID {
		case 100:
			if strings.Join(row.TagList, ",") != "prod,web" {
				t.Errorf("VM 100 tag_list = %v", row.TagList)
			}
		case 102:
			if row.TagList == nil || len(row.TagList) != 0 {
				t.Errorf("VM 102 has no tags; tag_list should be [] not %#v", row.TagList)
			}
		}
	}
}

func TestIPAddressIsOnlyPresentWhenKnown(t *testing.T) {
	r := run(t, fakepve.New(t), "", "ct", "list", "-o", "json")
	var rows []map[string]interface{}
	_ = json.Unmarshal([]byte(r.stdout), &rows)
	for _, row := range rows {
		ip, has := row["ip_address"]
		switch row["name"] {
		case "ct1": // running with an address
			if !has || ip != "10.0.0.50" || row["ip"] != "10.0.0.50" {
				t.Errorf("ct1: ip_address %v, ip %v", ip, row["ip"])
			}
		case "ct2": // stopped: unknown
			if has || row["ip"] != "N/A" {
				t.Errorf("ct2 should have no ip_address and ip N/A: %v", row)
			}
		}
	}
}

func taskResult(t *testing.T, stdout string) map[string]interface{} {
	t.Helper()
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout should be one JSON object: %v\n%s", err, stdout)
	}
	return got
}

func TestTaskCommandsPrintOneJSONObjectWithOutputJSON(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "vm", "start", "100", "--wait", "-o", "json")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	got := taskResult(t, r.stdout)
	if got["task"] != upid || got["node"] != "pve1" || got["waited"] != true || got["status"] != "ok" || got["message"] != "VM web (ID 100) started" {
		t.Errorf("result = %v", got)
	}
	if secs, _ := got["seconds"].(float64); secs < 0 {
		t.Errorf("seconds = %v", got["seconds"])
	}
	excludes(t, "stdout", r.stdout, "Task started", " in ")
}

func TestTaskCommandsWithoutWaitSayTheTaskStarted(t *testing.T) {
	r := run(t, fakepve.New(t), "", "vm", "start", "100", "-o", "json")
	got := taskResult(t, r.stdout)
	if got["task"] != upid || got["waited"] != false || got["status"] != "started" {
		t.Errorf("result = %v", got)
	}
	if _, has := got["message"]; has {
		t.Errorf("nothing has finished, so there is no message: %v", got)
	}
	excludes(t, "stderr", r.stderr, "Tip:") // progress hints are text-mode only
}

func TestTaskResultsAreAvailableAsYAMLToo(t *testing.T) {
	r := run(t, fakepve.New(t), "", "ct", "stop", "ct1", "-o", "yaml")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	contains(t, "stdout", r.stdout, "task: '"+upid+"'", "node: pve1", "status: ok", "waited: true")
}

func TestEveryTaskCommandSupportsStructuredResults(t *testing.T) {
	for _, args := range [][]string{
		{"vm", "start", "100"},
		{"vm", "shutdown", "100", "--wait"},
		{"vm", "reboot", "100", "--wait"},
		{"vm", "delete", "100", "--yes", "--wait"},
		{"vm", "clone", "100", "web3", "--wait"},
		{"vm", "migrate", "100", "pve2"},
		{"vm", "snapshot", "create", "100", "before"},
		{"ct", "start", "ct2"},
		{"ct", "stop", "ct1"},
		{"ct", "delete", "ct1", "--yes"},
		{"ct", "clone", "ct1", "ct1-copy", "--full"},
		{"ct", "snapshot", "delete", "ct1", "snap1", "--yes"},
	} {
		r := run(t, fakepve.New(t), "", append(args, "-o", "json")...)
		if r.code != 0 {
			t.Errorf("prox %v: exit %d: %s", args, r.code, r.stderr)
			continue
		}
		if got := taskResult(t, r.stdout); got["task"] != upid {
			t.Errorf("prox %v: result = %v", args, got)
		}
	}
}

func TestAFailedTaskStillFailsInStructuredMode(t *testing.T) {
	srv := fakepve.New(t)
	srv.FailRequest("POST /nodes/pve1/qemu/101/status/start", "VM 101 is locked (backup)")
	r := run(t, srv, "", "vm", "start", "db", "-o", "json")
	if r.code != 1 || strings.TrimSpace(r.stdout) != "" {
		t.Errorf("exit %d, stdout %q; want exit 1 and nothing on stdout", r.code, r.stdout)
	}
	contains(t, "stderr", r.stderr, "VM 101 is locked")
}

func TestTextModeIsUnchangedByTheStructuredOption(t *testing.T) {
	r := run(t, fakepve.New(t), "", "vm", "start", "100", "--wait")
	contains(t, "stdout", r.stdout, "Task started: "+upid, "VM web (ID 100) started in ")
	if strings.HasPrefix(strings.TrimSpace(r.stdout), "{") {
		t.Errorf("text mode printed JSON:\n%s", r.stdout)
	}
}
