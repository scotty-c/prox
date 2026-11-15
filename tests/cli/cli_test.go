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
	excludes(t, "ct list", r.stdout, "IP")
}

func TestContainerListTableWithIPs(t *testing.T) {
	r := run(t, fakepve.New(t), "", "ct", "list", "--ip")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	golden(t, "ct_list_ip_table", r.stdout)
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
	cmd.Env = []string{"HOME=" + t.TempDir(), "PATH=" + os.Getenv("PATH"), "PROX_CREDENTIAL_STORE=file"}
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
	r := run(t, fakepve.New(t), "", "vm", "start", "100", "--no-wait")
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
	r := runInHome(t, home, srv, "", "config", "setup", "--no-verify", "lab", "-u", "root@pam")
	if r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "missing --password, --url")
	if got := profiles(t, home, srv); len(got) != 0 {
		t.Errorf("a profile was saved anyway: %v", got)
	}
	// no flags at all
	if r := runInHome(t, home, srv, "", "config", "setup", "--no-verify", "lab"); r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	if got := profiles(t, home, srv); len(got) != 0 {
		t.Errorf("a profile was saved anyway: %v", got)
	}
}

func TestConfigCreateSavesAProfileThatHasEverything(t *testing.T) {
	home, srv := t.TempDir(), fakepve.New(t)
	r := runInHome(t, home, srv, "", "config", "setup", "--no-verify", "lab", "-u", "root@pam", "-p", "pw", "-l", "https://pve:8006")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	if got := profiles(t, home, srv); len(got) != 1 || got[0] != "lab" {
		t.Errorf("profiles = %v", got)
	}
}

func TestConfigDeleteAsksFirstAndNeedsAnAnswer(t *testing.T) {
	home, srv := t.TempDir(), fakepve.New(t)
	runInHome(t, home, srv, "", "config", "setup", "--no-verify", "lab", "-u", "u@pam", "-p", "pw", "-l", "https://pve:8006")

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
	runInHome(t, home, srv, "", "config", "setup", "--no-verify", "default", "-u", "u@pam", "-p", "pw", "-l", "https://pve:8006")
	runInHome(t, home, srv, "", "config", "setup", "--no-verify", "other", "-u", "u@pam", "-p", "pw", "-l", "https://pve:8006")

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
	runInHome(t, home, srv, "", "config", "setup", "--no-verify", "lab", "-u", "u@pam", "-p", "pw", "-l", "https://pve:8006")
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
	r := run(t, fakepve.New(t), "", "ct", "list", "--ip", "-o", "json")
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
	r := run(t, fakepve.New(t), "", "vm", "start", "100", "--no-wait", "-o", "json")
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

// ct create --node

func TestContainerCreateOnANodeWithoutTheTemplateFailsInsteadOfMovingNodes(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "ct", "create", "web2", "debian:12", "--node", "pve2")
	if r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "is not on node pve2", "it is on pve1", "prox ct templates pull", "--node pve2")
	if srv.Count("POST", "/nodes/") != 0 {
		t.Errorf("nothing should have been created: %v", srv.Requests())
	}
}

func TestContainerCreateOnTheNodeThatHasTheTemplateWorks(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "ct", "create", "web2", "debian:12", "--node", "pve1")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	if srv.Count("POST", "/nodes/pve1/lxc") != 1 {
		t.Errorf("container should be created on pve1: %v", srv.Requests())
	}
}

// prox tasks

func TestTasksListsRecentTasksNewestFirst(t *testing.T) {
	r := run(t, fakepve.New(t), "", "tasks")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	contains(t, "tasks", r.stdout, "vzdump", "qmclone", "qmstart", "running", "OK", "failed: clone failed: storage 'local-lvm' is not online", "backup@pve")
	if !(strings.Index(r.stdout, "vzdump") < strings.Index(r.stdout, "qmclone") && strings.Index(r.stdout, "qmclone") < strings.Index(r.stdout, "qmstart")) {
		t.Errorf("tasks should be newest first:\n%s", r.stdout)
	}
	contains(t, "stderr", r.stderr, "--upid")
	excludes(t, "stdout", r.stdout, "UPID:pve1")
}

func TestTasksCanShowTheTaskID(t *testing.T) {
	r := run(t, fakepve.New(t), "", "tasks", "--upid", "--limit", "1")
	contains(t, "tasks", r.stdout, "UPID:pve2:00000002:00000002:6A000100:vzdump:101:root@pam:", "Showing 1 task(s)")
	excludes(t, "stderr", r.stderr, "--upid") // the hint is only for when the IDs are hidden
}

func TestTasksFilters(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "tasks", "--running")
	contains(t, "running", r.stdout, "vzdump")
	excludes(t, "running", r.stdout, "qmstart", "qmclone")

	r = run(t, srv, "", "tasks", "--failed")
	contains(t, "failed", r.stdout, "qmclone")
	excludes(t, "failed", r.stdout, "vzdump", "qmstart")

	r = run(t, srv, "", "tasks", "--node", "pve2")
	contains(t, "node", r.stdout, "vzdump")
	excludes(t, "node", r.stdout, "qmstart")

	r = run(t, srv, "", "tasks", "--failed", "--node", "pve2")
	if r.code != 0 || strings.TrimSpace(r.stdout) != "" {
		t.Errorf("nothing matches: exit %d, stdout %q", r.code, r.stdout)
	}
	contains(t, "stderr", r.stderr, "No matching tasks")
}

func TestTasksJSONIncludesTheTaskIDAndStatus(t *testing.T) {
	r := run(t, fakepve.New(t), "", "tasks", "-o", "json")
	var rows []struct {
		UPID, Node, Type, Target, Status, Message string
		StartTime                                 int64 `json:"start_time"`
		EndTime                                   int64 `json:"end_time"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &rows); err != nil {
		t.Fatalf("%v\n%s", err, r.stdout)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d tasks: %v", len(rows), rows)
	}
	want := []struct{ typ, status string }{{"vzdump", "running"}, {"qmclone", "failed"}, {"qmstart", "ok"}}
	for i, w := range want {
		if rows[i].Type != w.typ || rows[i].Status != w.status || !strings.HasPrefix(rows[i].UPID, "UPID:") {
			t.Errorf("row %d = %+v, want %s/%s", i, rows[i], w.typ, w.status)
		}
	}
	if rows[0].EndTime != 0 || rows[1].Message != "clone failed: storage 'local-lvm' is not online" {
		t.Errorf("running task has no end time, failed task carries Proxmox's message: %+v", rows)
	}
}

// ssh config markers and --prune

const marker = "# managed by prox"

// sshHome makes a home directory with the given ~/.ssh/config
func sshHome(t *testing.T, config string) string {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(home+"/.ssh", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(home+"/.ssh/config", []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	return home
}

func readSSHConfig(t *testing.T, home string) string {
	t.Helper()
	data, err := os.ReadFile(home + "/.ssh/config")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

const mixedConfig = `Host legacy
    HostName 10.9.9.9

Host gone
    ` + marker + `
    HostName 10.0.0.9

Host web
    ` + marker + `
    HostName 10.0.0.2

# my own notes
Host gone-by-hand
    HostName 10.8.8.8
`

func TestSSHWritesAMarkerAndListShowsWhichEntriesAreProxs(t *testing.T) {
	home := sshHome(t, "Host legacy\n    HostName 10.9.9.9\n")
	srv := fakepve.New(t)
	if r := runInHome(t, home, srv, "", "ssh", "ct1", "--user", "root"); r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	got := readSSHConfig(t, home)
	contains(t, "ssh config", got, "Host ct1\n    "+marker+"\n    HostName 10.0.0.50")

	r := runInHome(t, home, srv, "", "ssh", "--list")
	var ct1Row, legacyRow string
	for _, line := range strings.Split(r.stdout, "\n") {
		switch {
		case strings.Contains(line, "ct1"):
			ct1Row = line
		case strings.Contains(line, "legacy"):
			legacyRow = line
		}
	}
	if !strings.Contains(ct1Row, "prox") || strings.Contains(legacyRow, "prox") {
		t.Errorf("only ct1 is prox's:\n%s", r.stdout)
	}

	// Running it again replaces the entry rather than piling up markers
	runInHome(t, home, srv, "", "ssh", "ct1", "--user", "admin")
	if n := strings.Count(readSSHConfig(t, home), marker); n != 1 {
		t.Errorf("%d markers after updating the entry, want 1:\n%s", n, readSSHConfig(t, home))
	}
}

func TestSSHPruneDryRunListsStaleEntriesAndChangesNothing(t *testing.T) {
	home := sshHome(t, mixedConfig)
	r := runInHome(t, home, fakepve.New(t), "", "ssh", "--prune", "--dry-run")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	contains(t, "stdout", r.stdout, "gone (10.0.0.9)", "Would remove 1 entry")
	excludes(t, "stdout", r.stdout, "web (", "legacy", "gone-by-hand")
	if readSSHConfig(t, home) != mixedConfig {
		t.Error("a dry run must not change the file")
	}
}

func TestSSHPruneRemovesOnlyStaleEntriesProxWrote(t *testing.T) {
	home := sshHome(t, mixedConfig)
	r := runInHome(t, home, fakepve.New(t), "", "ssh", "--prune", "--yes")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	got := readSSHConfig(t, home)
	excludes(t, "ssh config", got, "Host gone\n", "10.0.0.9")
	contains(t, "ssh config", got, "Host legacy", "Host web", "Host gone-by-hand", "# my own notes")
	if _, err := os.Stat(home + "/.ssh/config.prox.bak"); err != nil {
		t.Errorf("a backup should be left: %v", err)
	}
	contains(t, "stdout", r.stdout, "Removed 1 entry")
}

func TestSSHPruneNeedsAnAnswerAndDoesNotAssumeYes(t *testing.T) {
	home := sshHome(t, mixedConfig)
	r := runInHome(t, home, fakepve.New(t), "", "ssh", "--prune")
	if r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "Remove 1 entry", "pass --yes")
	if readSSHConfig(t, home) != mixedConfig {
		t.Error("nothing should change without an answer")
	}
	if r := runInHome(t, home, fakepve.New(t), "n\n", "ssh", "--prune"); r.code != 0 || readSSHConfig(t, home) != mixedConfig {
		t.Errorf("answering no: exit %d, config changed: %v", r.code, readSSHConfig(t, home) != mixedConfig)
	}
}

func TestSSHPruneNeverGuessesWhenTheClusterIsUnreachable(t *testing.T) {
	home := sshHome(t, mixedConfig)
	srv := fakepve.New(t)
	srv.Close()
	r := runInHome(t, home, srv, "", "ssh", "--prune", "--yes")
	if r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "nothing was changed")
	if readSSHConfig(t, home) != mixedConfig {
		t.Error("an unreachable cluster must not look like every guest is gone")
	}
}

func TestSSHPruneRejectsArgumentsAndOtherModes(t *testing.T) {
	for _, args := range [][]string{{"ssh", "--prune", "web"}, {"ssh", "--prune", "--list"}, {"ssh", "--prune", "--delete"}} {
		r := runInHome(t, sshHome(t, mixedConfig), fakepve.New(t), "", args...)
		if r.code != 1 {
			t.Errorf("prox %v: exit %d, want 1", args, r.code)
		}
		contains(t, "stderr", r.stderr, "--prune takes no arguments")
	}
}

// ct shortcuts and templates pull

func TestShortcutsComeFromWhatTheClusterCanDownload(t *testing.T) {
	r := run(t, fakepve.New(t), "", "ct", "shortcuts")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	contains(t, "shortcuts", r.stdout, "debian:12", "debian:11", "ubuntu:24.04", "ubuntu:22.04", "alpine:3.20", "alpine:3.9")
	excludes(t, "shortcuts", r.stdout, "turnkey", "wordpress", "alpine:3.18", "fedora:38", "centos:8")
	order := func(s string) int { return strings.Index(r.stdout, s) }
	if !(order("ubuntu:24.04") < order("ubuntu:22.04") && order("alpine:3.20") < order("alpine:3.9")) {
		t.Errorf("newest version should come first within an OS:\n%s", r.stdout)
	}
	// debian 12 is on the node (see the fake's storage); the others are not
	for _, line := range strings.Split(r.stdout, "\n") {
		switch {
		case strings.Contains(line, "debian:12"):
			contains(t, "debian:12 row", line, "yes")
		case strings.Contains(line, "ubuntu:24.04"):
			contains(t, "ubuntu:24.04 row", line, "no")
		}
	}
}

func TestShortcutsJSON(t *testing.T) {
	r := run(t, fakepve.New(t), "", "ct", "shortcuts", "-o", "json")
	var rows []struct {
		Shortcut, Template string
		Downloaded         bool
	}
	if err := json.Unmarshal([]byte(r.stdout), &rows); err != nil {
		t.Fatalf("%v\n%s", err, r.stdout)
	}
	found := false
	for _, row := range rows {
		if row.Shortcut == "debian:12" {
			found = true
			if row.Template != "debian-12-standard_12.7-1_amd64.tar.zst" || !row.Downloaded {
				t.Errorf("debian:12 = %+v", row)
			}
		}
	}
	if !found {
		t.Errorf("no debian:12 in %+v", rows)
	}
}

func TestShortcutsFallBackToExamplesWhenTheClusterIsUnreachable(t *testing.T) {
	srv := fakepve.New(t)
	srv.Close()
	r := run(t, srv, "", "ct", "shortcuts")
	if r.code != 0 {
		t.Errorf("exit %d, want 0: the examples still help", r.code)
	}
	contains(t, "stderr", r.stderr, "Could not ask Proxmox", "showing common examples")
	contains(t, "stdout", r.stdout, "debian:12", "ubuntu:24.04")
}

func TestTemplatePullDownloadsAShortcutAndSkipsOnesThatAreThere(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "ct", "templates", "pull", "ubuntu:24.04", "--wait")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	body := srv.Body("POST /nodes/pve1/aplinfo")
	contains(t, "download request", body, `"template":"ubuntu-24.04-standard_24.04-2_amd64.tar.zst"`, `"storage":"local"`)

	srv2 := fakepve.New(t)
	r = run(t, srv2, "", "ct", "templates", "pull", "debian:12")
	contains(t, "stdout", r.stdout, "is already on local")
	if srv2.Count("POST", "/nodes/pve1/aplinfo") != 0 {
		t.Errorf("nothing should be downloaded again: %v", srv2.Requests())
	}
}

// config: --password-stdin, URL tidying, richer list

func TestConfigListShowsURLAndUserButNeverThePassword(t *testing.T) {
	home, srv := t.TempDir(), fakepve.New(t)
	runInHome(t, home, srv, "", "config", "setup", "--no-verify", "lab", "-u", "root@pam", "-p", "s3cret-lab", "-l", "https://lab.example.com:8006")
	runInHome(t, home, srv, "", "config", "setup", "--no-verify", "prod", "-u", "ci@pve!deploy", "-p", "s3cret-prod", "-l", "https://prod.example.com:8006", "--use")

	r := runInHome(t, home, srv, "", "config", "list")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	contains(t, "list", r.stdout, "lab", "https://lab.example.com:8006", "root@pam", "prod", "https://prod.example.com:8006", "ci@pve!deploy")
	excludes(t, "list", r.stdout+r.stderr, "s3cret")
	for _, line := range strings.Split(r.stdout, "\n") {
		if strings.Contains(line, "prod") && !strings.Contains(line, "*") {
			t.Errorf("the profile in use should be marked: %q", line)
		}
		if strings.Contains(line, "lab.example.com") && strings.Contains(line, "*") {
			t.Errorf("lab is not in use: %q", line)
		}
	}

	r = runInHome(t, home, srv, "", "config", "list", "-o", "json")
	var rows []struct {
		Name, URL, User string
		Current         bool
	}
	if err := json.Unmarshal([]byte(r.stdout), &rows); err != nil || len(rows) != 2 {
		t.Fatalf("%v\n%s", err, r.stdout)
	}
	excludes(t, "json", r.stdout, "s3cret", "password")
	for _, row := range rows {
		if row.Name == "prod" && (!row.Current || row.User != "ci@pve!deploy" || row.URL != "https://prod.example.com:8006") {
			t.Errorf("prod = %+v", row)
		}
	}
}

func TestPasswordStdinKeepsThePasswordOffTheCommandLine(t *testing.T) {
	home, srv := t.TempDir(), fakepve.New(t)
	r := runInHome(t, home, srv, "from-stdin\n", "config", "setup", "--no-verify", "lab", "-u", "u@pam", "-l", "https://pve:8006", "--password-stdin")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	r = runInHome(t, home, srv, "", "config", "read", "--profile", "lab", "--show-password")
	contains(t, "config read", r.stdout, "from-stdin")

	// setup on the existing profile can change it the same way
	runInHome(t, home, srv, "", "config", "use", "lab")
	if r := runInHome(t, home, srv, "second-secret\n", "config", "setup", "--no-verify", "--password-stdin"); r.code != 0 {
		t.Fatalf("setup: exit %d: %s", r.code, r.stderr)
	}
	r = runInHome(t, home, srv, "", "config", "read", "--show-password")
	contains(t, "config read", r.stdout, "second-secret")
}

func TestPasswordStdinMistakesAreReported(t *testing.T) {
	srv := fakepve.New(t)
	r := runInHome(t, t.TempDir(), srv, "x\n", "config", "setup", "--no-verify", "lab", "-u", "u@pam", "-l", "https://pve:8006", "-p", "a", "--password-stdin")
	if r.code != 1 {
		t.Errorf("both given: exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "either --password or --password-stdin")

	home := t.TempDir()
	r = runInHome(t, home, srv, "", "config", "setup", "--no-verify", "lab", "-u", "u@pam", "-l", "https://pve:8006", "--password-stdin")
	if r.code != 1 {
		t.Errorf("nothing on stdin: exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "no password received on stdin")
	if got := profiles(t, home, srv); len(got) != 0 {
		t.Errorf("nothing should be saved: %v", got)
	}

	r = runInHome(t, home, srv, "\n", "config", "setup", "--no-verify", "lab", "-u", "u@pam", "-l", "https://pve:8006", "--password-stdin")
	contains(t, "stderr", r.stderr, "is empty")
}

func TestURLsAreTidiedBeforeTheyAreSaved(t *testing.T) {
	srv := fakepve.New(t)
	for in, want := range map[string]string{
		"pve.example.com:8006":                   "https://pve.example.com:8006",
		"https://pve.example.com:8006/":          "https://pve.example.com:8006",
		"https://pve.example.com:8006/api2/json": "https://pve.example.com:8006",
	} {
		home := t.TempDir()
		if r := runInHome(t, home, srv, "", "config", "setup", "--no-verify", "lab", "-u", "u@pam", "-p", "pw", "-l", in); r.code != 0 {
			t.Fatalf("%q: exit %d: %s", in, r.code, r.stderr)
		}
		r := runInHome(t, home, srv, "", "config", "read", "--profile", "lab")
		contains(t, "saved URL for "+in, r.stdout, want)
	}
}

func TestURLWithoutAPortGetsANoteButIsNotChanged(t *testing.T) {
	home, srv := t.TempDir(), fakepve.New(t)
	r := runInHome(t, home, srv, "", "config", "setup", "--no-verify", "lab", "-u", "u@pam", "-p", "pw", "-l", "https://pve.example.com")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	contains(t, "stderr", r.stderr, "Note:", "8006", "reverse proxy")
	contains(t, "config read", runInHome(t, home, srv, "", "config", "read", "--profile", "lab").stdout, "https://pve.example.com")
}

func TestAnUnusableURLIsRejectedBeforeAnythingIsSaved(t *testing.T) {
	home, srv := t.TempDir(), fakepve.New(t)
	r := runInHome(t, home, srv, "", "config", "setup", "--no-verify", "lab", "-u", "u@pam", "-p", "pw", "-l", "ftp://pve:8006")
	if r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "must start with https://")
	if got := profiles(t, home, srv); len(got) != 0 {
		t.Errorf("nothing should be saved: %v", got)
	}
}

func TestConfigHelpHasRealDescriptions(t *testing.T) {
	r := run(t, fakepve.New(t), "", "config", "--help")
	excludes(t, "config help", r.stdout, "This command")
	contains(t, "config help", r.stdout, "Create a connection profile, or change a saved one")
	excludes(t, "config help", r.stdout, "Update the", "  create ", "  update ")
}

// Requests per command

// gets counts GET requests starting with prefix
func gets(srv *fakepve.Server, prefix string) int { return srv.Count("GET", prefix) }

func TestDescribeFetchesEachThingOnce(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "vm", "describe", "100")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	if n := gets(srv, "/nodes/pve1/qemu/100/config"); n != 1 {
		t.Errorf("config fetched %d times, want 1", n)
	}
	if n := gets(srv, "/nodes/pve1/qemu/100/status/current"); n != 1 {
		t.Errorf("status fetched %d times, want 1", n)
	}
	if total := len(srv.Requests()); total > 6 {
		t.Errorf("%d requests, want at most 6 (login, listing, config, status, two guest-agent calls): %v", total, srv.Requests())
	}
	// the page still has everything it had
	contains(t, "describe", r.stdout, "SCSI0:", "QEMU Agent: Enabled", "Disk Usage: N/A / 132.0 GB")
}

func TestDetailedListSumsEveryDisksSizeAndAsksStoppedVMsForLess(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "vm", "list", "--detailed")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	// The total comes from the configuration (all disks), for stopped VMs too:
	// the cluster listing only knows the boot disk. Every VM here reports the
	// fake's two-disk configuration.
	if n := strings.Count(r.stdout, "N/A/132.0 GB"); n != 4 {
		t.Errorf("%d of 4 VMs show the summed disk size, want all:\n%s", n, r.stdout)
	}
	// A stopped VM has no status to read and no guest agent to ask
	for _, stopped := range []string{"/nodes/pve1/qemu/101", "/nodes/pve2/qemu/102"} {
		if n := gets(srv, stopped+"/config"); n != 1 {
			t.Errorf("%s config fetched %d times, want 1", stopped, n)
		}
		if n := gets(srv, stopped+"/status"); n != 0 {
			t.Errorf("%s status fetched %d times, want 0", stopped, n)
		}
		if n := gets(srv, stopped+"/agent"); n != 0 {
			t.Errorf("%s guest agent asked %d times, want 0", stopped, n)
		}
	}
	// A running VM is asked for its config, status and guest agent once each
	for _, running := range []string{"/nodes/pve1/qemu/100", "/nodes/pve2/qemu/1000"} {
		for _, part := range []string{"/config", "/status/current", "/agent/get-fsinfo"} {
			if n := gets(srv, running+part); n != 1 {
				t.Errorf("%s fetched %d times, want 1", running+part, n)
			}
		}
	}
	if total := len(srv.Requests()); total > 10 {
		t.Errorf("%d requests, want at most 10: %v", total, srv.Requests())
	}
}

func TestCreatingOnAChosenNodeOnlyAsksThatNodeForTemplates(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "ct", "create", "web2", "debian:12", "--node", "pve1")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	if n := gets(srv, "/nodes/pve2/storage"); n != 0 {
		t.Errorf("pve2 was asked about storage %d times; the chosen node already has the template", n)
	}
}

func TestWithoutANodeEveryNodeIsAskedForTemplates(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "ct", "create", "web2", "debian:12")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	for _, node := range []string{"pve1", "pve2"} {
		if n := gets(srv, "/nodes/"+node+"/storage"); n == 0 {
			t.Errorf("%s was never asked for templates", node)
		}
	}
	contains(t, "stdout", r.stdout, "Container web2")
}

func TestAMistypedNodeStillExplainsWhereTheTemplateIs(t *testing.T) {
	r := run(t, fakepve.New(t), "", "ct", "create", "web2", "debian:12", "--node", "nope")
	if r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "is not on node nope", "it is on pve1")
}

// Waiting is the default for every command that starts a task

func taskPolls(srv *fakepve.Server) int { return srv.Count("GET", "/nodes/pve1/tasks/") }

func TestVMOperationsWaitForTheTaskByDefault(t *testing.T) {
	for _, tc := range []struct {
		args []string
		done string
	}{
		{[]string{"vm", "start", "100"}, "VM web (ID 100) started in "},
		{[]string{"vm", "shutdown", "100"}, "VM web (ID 100) shut down in "},
		{[]string{"vm", "stop", "100"}, "VM web (ID 100) shut down in "},
		{[]string{"vm", "reboot", "100"}, "VM web (ID 100) rebooted in "},
		{[]string{"vm", "delete", "100", "--yes"}, "VM web (ID 100) deleted in "},
	} {
		srv := fakepve.New(t)
		r := run(t, srv, "", tc.args...)
		if r.code != 0 {
			t.Errorf("prox %v: exit %d: %s", tc.args, r.code, r.stderr)
			continue
		}
		if n := taskPolls(srv); n == 0 {
			t.Errorf("prox %v never checked on its task: it should wait by default", tc.args)
		}
		contains(t, strings.Join(tc.args, " "), r.stdout, "Task started: "+upid, tc.done)
	}
}

func TestNoWaitReturnsOnceTheTaskHasStarted(t *testing.T) {
	for _, args := range [][]string{
		{"vm", "start", "100"}, {"vm", "shutdown", "100"}, {"vm", "reboot", "100"}, {"vm", "delete", "100", "--yes"},
	} {
		srv := fakepve.New(t)
		r := run(t, srv, "", append(args, "--no-wait")...)
		if r.code != 0 {
			t.Errorf("prox %v: exit %d: %s", args, r.code, r.stderr)
			continue
		}
		if n := taskPolls(srv); n != 0 {
			t.Errorf("prox %v --no-wait polled the task %d times", args, n)
		}
		contains(t, "stdout", r.stdout, "Task started: "+upid)
		contains(t, "stderr", r.stderr, "prox logs --follow "+upid)
		excludes(t, "stdout", r.stdout, " in ")
	}
}

func TestTheOldWaitFlagsStillWork(t *testing.T) {
	// scripts written when waiting was opt-in keep working unchanged
	for _, flag := range []string{"--wait", "-w"} {
		srv := fakepve.New(t)
		r := run(t, srv, "", "vm", "start", "100", flag)
		if r.code != 0 || taskPolls(srv) == 0 {
			t.Errorf("vm start %s: exit %d, polls %d\n%s", flag, r.code, taskPolls(srv), r.stderr)
		}
	}
}

func TestVMCreateWaitsForTheVMToStartByDefault(t *testing.T) {
	waiting, early := fakepve.New(t), fakepve.New(t)
	if r := run(t, waiting, "", "vm", "create", "web3", "--from", "100", "--start"); r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	if r := run(t, early, "", "vm", "create", "web3", "--from", "100", "--start", "--no-wait"); r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	// the clone is always waited for; the start only by default
	if taskPolls(waiting) != taskPolls(early)+1 {
		t.Errorf("polls: default %d, --no-wait %d; the default should also wait for the start", taskPolls(waiting), taskPolls(early))
	}
}

func TestBulkOperationsWaitForEachTarget(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "vm", "start", "--tag", "prod", "--yes")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	// two VMs carry the tag prod (web and db)
	if n := taskPolls(srv); n != 2 {
		t.Errorf("polled %d times for 2 targets, want 2", n)
	}
	contains(t, "stdout", r.stdout, "VM web (ID 100) started in ", "VM db (ID 101) started in ")
}

func TestWaitDefaultsAreTheSameEverywhere(t *testing.T) {
	// Every command that starts a task says (default) beside --wait, not --no-wait
	for _, args := range [][]string{
		{"vm", "start"}, {"vm", "shutdown"}, {"vm", "reboot"}, {"vm", "delete"}, {"vm", "create"}, {"vm", "clone"}, {"vm", "migrate"},
		{"ct", "start"}, {"ct", "stop"}, {"ct", "delete"}, {"ct", "create"}, {"ct", "clone"}, {"ct", "migrate"}, {"ct", "reboot"},
		{"vm", "snapshot", "create"}, {"ct", "templates", "pull"},
	} {
		r := run(t, fakepve.New(t), "", append(args, "--help")...)
		var waitLine, noWaitLine string
		for _, line := range strings.Split(r.stdout, "\n") {
			if strings.Contains(line, "--wait") {
				waitLine = line
			}
			if strings.Contains(line, "--no-wait") {
				noWaitLine = line
			}
		}
		if !strings.Contains(waitLine, "(default)") || strings.Contains(noWaitLine, "(default)") {
			t.Errorf("prox %v: --wait should be the default\n  %s\n  %s", args, waitLine, noWaitLine)
		}
	}
}

// Container addresses come from /interfaces alone: a real node has no "ips" in
// the container status and no guest agent endpoint for containers (checked on
// Proxmox, which answers "No 'get' handler defined"), so asking would only
// add requests that cannot succeed.
func TestContainerAddressesNeedOneRequestEach(t *testing.T) {
	srv := fakepve.New(t)
	r := run(t, srv, "", "ct", "list", "--ip")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	contains(t, "ct list --ip", r.stdout, "10.0.0.50")
	if n := gets(srv, "/nodes/pve1/lxc/200/interfaces"); n != 1 {
		t.Errorf("%d interface requests for container 200, want 1", n)
	}
	for _, prefix := range []string{"/nodes/pve1/lxc/200/status/current", "/nodes/pve1/lxc/200/agent"} {
		if n := gets(srv, prefix); n != 0 {
			t.Errorf("ct list asked %s %d times; it cannot give an address", prefix, n)
		}
	}

	srv = fakepve.New(t)
	if r := run(t, srv, "", "ssh", "ct1", "--print"); r.code != 0 {
		t.Fatalf("ssh: exit %d: %s", r.code, r.stderr)
	}
	if n := gets(srv, "/nodes/pve1/lxc/200/agent"); n != 0 {
		t.Errorf("prox ssh asked a container's guest agent %d times; containers have none", n)
	}
}

// vm list and ct list work the same way: addresses cost a request per guest,
// so they are fetched, shown and put in JSON only when asked for with --ip
func TestVMAndContainerListsHandleAddressesTheSame(t *testing.T) {
	for _, kind := range []struct{ cmd, lookup, name string }{
		{"vm", "/qemu/", "web"},
		{"ct", "/lxc/", "ct1"},
	} {
		// default: no IP column, no field in JSON, no per-guest lookups
		srv := fakepve.New(t)
		r := run(t, srv, "", kind.cmd, "list")
		if r.code != 0 {
			t.Fatalf("%s list: exit %d: %s", kind.cmd, r.code, r.stderr)
		}
		excludes(t, kind.cmd+" list", r.stdout, " IP ")
		r = run(t, srv, "", kind.cmd, "list", "-o", "json")
		excludes(t, kind.cmd+" list -o json", r.stdout, "ip_address")
		for _, req := range srv.Requests() {
			if strings.Contains(req, kind.lookup) {
				t.Errorf("%s list made a per-guest request without --ip: %s", kind.cmd, req)
			}
		}

		// -i and --ip both turn it on
		for _, flag := range []string{"-i", "--ip"} {
			r = run(t, fakepve.New(t), "", kind.cmd, "list", flag)
			if r.code != 0 {
				t.Fatalf("%s list %s: exit %d: %s", kind.cmd, flag, r.code, r.stderr)
			}
			contains(t, kind.cmd+" list "+flag, r.stdout, " IP ")
		}
	}
}
