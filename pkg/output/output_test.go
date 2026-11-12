package output

import (
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout runs fn and returns what it wrote to stdout
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestOutputFormats(t *testing.T) {
	defer func() { _ = SetFormat(FormatTable) }()
	data := []struct {
		MaxMemory uint64 `json:"max_memory"`
		Name      string `json:"name"`
	}{{2048, "web"}}

	if err := SetFormat("xml"); err == nil {
		t.Fatal("expected error for unknown format")
	}

	_ = SetFormat(FormatJSON)
	got := captureStdout(t, func() { _ = OutputJSON(data) })
	if !strings.Contains(got, `"max_memory": 2048`) {
		t.Errorf("json output: %s", got)
	}

	_ = SetFormat(FormatYAML)
	got = captureStdout(t, func() { _ = OutputJSON(data) })
	if want := "- max_memory: 2048\n  name: web\n"; got != want {
		t.Errorf("yaml output = %q, want %q (keys must match JSON)", got, want)
	}

	// Progress messages must not mix into structured output
	if got := captureStdout(t, func() { Info("Retrieving...\n") }); got != "" {
		t.Errorf("Info printed %q in yaml mode", got)
	}
	_ = SetFormat(FormatTable)
	if got := captureStdout(t, func() { Info("Retrieving...\n") }); got != "Retrieving...\n" {
		t.Errorf("Info in table mode = %q", got)
	}
}
