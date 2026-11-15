package output

import (
	"bytes"
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

func TestConfirm(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    bool
		wantErr bool
	}{
		{"yes", "y\n", true, false},
		{"yes word, any case", "YES\n", true, false},
		{"no", "n\n", false, false},
		{"just enter", "\n", false, false},
		{"junk", "maybe\n", false, false},
		{"answer without newline", "y", true, false},
		{"closed stdin", "", false, true},
		{"whitespace then EOF", "  ", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			got, err := confirmFrom(strings.NewReader(tt.input), &out, "Delete?", "--force")
			if got != tt.want || (err != nil) != tt.wantErr {
				t.Fatalf("confirmFrom(%q) = %v, %v; want %v, err=%v", tt.input, got, err, tt.want, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), "--force") {
				t.Fatalf("error should name the skip flag: %v", err)
			}
		})
	}
}
