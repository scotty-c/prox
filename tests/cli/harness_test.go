// Package cli runs the real prox binary against a fake Proxmox API and checks
// what a user would see: output, exit codes and the requests that were made.
package cli

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/scotty-c/prox/internal/fakepve"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata/")

var binary string

func TestMain(m *testing.M) {
	flag.Parse()
	dir, err := os.MkdirTemp("", "prox-cli-test")
	if err != nil {
		panic(err)
	}
	binary = filepath.Join(dir, "prox")
	build := exec.Command("go", "build", "-o", binary, "../..")
	if out, err := build.CombinedOutput(); err != nil {
		os.Stderr.Write(out)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// TestSourcesAreTracked makes Go's test cache notice source changes. The
// tests exercise a binary built by a subprocess, which the cache cannot see,
// so it would report a stale "ok" for edited code. Files a test opens are
// part of its cache key (but only once m.Run has started, so this cannot be
// done in TestMain), hence this test reads every source file in the module.
func TestSourcesAreTracked(t *testing.T) {
	readSources("../..")
}

func readSources(root string) {
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == ".beads") {
			return filepath.SkipDir
		}
		if strings.HasSuffix(path, ".go") || d.Name() == "go.mod" || d.Name() == "go.sum" {
			_, _ = os.ReadFile(path)
		}
		return nil
	})
}

// result is what one run of prox produced
type result struct {
	stdout, stderr string
	code           int
}

// run executes prox against srv with an empty home directory. stdin is what
// the command can read; "" behaves like </dev/null.
func run(t *testing.T, srv *fakepve.Server, stdin string, args ...string) result {
	t.Helper()
	return runInHome(t, t.TempDir(), srv, stdin, args...)
}

// runInHome is run with a chosen home directory, for tests that need files in
// it such as ~/.ssh/config
func runInHome(t *testing.T, home string, srv *fakepve.Server, stdin string, args ...string) result {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Env = []string{
		"HOME=" + home,
		"PATH=" + os.Getenv("PATH"),
		"PROX_URL=" + srv.URL,
		"PROX_USER=root@pam",
		"PROX_PASSWORD=secret",
		"PROX_CREDENTIAL_STORE=file", // never touch the real keychain
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
	}
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("running prox %v: %v", args, err)
	}
	return result{stdout.String(), stderr.String(), code}
}

// golden compares got with testdata/<name>.golden, or rewrites it with -update
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if got != string(want) {
		t.Errorf("output differs from %s (run with -update to accept):\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}

func contains(t *testing.T, what, s string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(s, w) {
			t.Errorf("%s should contain %q, got:\n%s", what, w, s)
		}
	}
}

func excludes(t *testing.T, what, s string, nots ...string) {
	t.Helper()
	for _, n := range nots {
		if strings.Contains(s, n) {
			t.Errorf("%s should not contain %q, got:\n%s", what, n, s)
		}
	}
}

// startInterrupted starts prox, waits until the fake server has received a
// request whose "METHOD /path" starts with waitFor (so prox is certainly in
// the middle of it), then sends Ctrl-C. It returns what prox did.
func startInterrupted(t *testing.T, srv *fakepve.Server, waitFor string, args ...string) (result, time.Duration) {
	t.Helper()
	home := t.TempDir()
	cmd := exec.Command(binary, args...)
	cmd.Env = []string{
		"HOME=" + home,
		"PATH=" + os.Getenv("PATH"),
		"PROX_URL=" + srv.URL,
		"PROX_USER=root@pam",
		"PROX_PASSWORD=secret",
		"PROX_CREDENTIAL_STORE=file", // never touch the real keychain
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	method, path, _ := strings.Cut(waitFor, " ")
	deadline := time.Now().Add(5 * time.Second)
	for srv.Count(method, path) == 0 {
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			t.Fatalf("prox never sent %q; requests so far: %v", waitFor, srv.Requests())
		}
		time.Sleep(10 * time.Millisecond)
	}

	sent := time.Now()
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		code := 0
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		}
		return result{stdout.String(), stderr.String(), code}, time.Since(sent)
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("prox did not stop within 5s of Ctrl-C; stderr:\n%s", stderr.String())
		return result{}, 0
	}
}

// runWithProfile runs prox with no PROX_URL/PROX_USER/PROX_PASSWORD, so that it
// uses the saved profile in home, as a real user's would
func runWithProfile(t *testing.T, home, stdin string, args ...string) result {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Env = []string{
		"HOME=" + home,
		"PATH=" + os.Getenv("PATH"),
		"PROX_CREDENTIAL_STORE=file", // never touch the real keychain
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
	}
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("running prox %v: %v", args, err)
	}
	return result{stdout.String(), stderr.String(), code}
}
