package util

import (
	"reflect"
	"testing"
)

func TestFormatSize(t *testing.T) {
	tests := map[uint64]string{
		0:                             "0 B",
		1023:                          "1023 B",
		1024:                          "1.0 KB",
		1536:                          "1.5 KB",
		32 * 1024 * 1024 * 1024:       "32.0 GB",
		5 * 1024 * 1024 * 1024 * 1024: "5.0 TB",
	}
	for in, want := range tests {
		if got := FormatSize(in); got != want {
			t.Errorf("FormatSize(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatUptime(t *testing.T) {
	tests := map[int64]string{
		-5:     "0s",
		0:      "0s",
		45:     "45s",
		125:    "2m 5s",
		3725:   "1h 2m 5s",
		200000: "2d 7h 33m",
	}
	for in, want := range tests {
		if got := FormatUptime(in); got != want {
			t.Errorf("FormatUptime(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestFindSimilarStrings(t *testing.T) {
	candidates := []string{"web-server", "web-server-2", "db-primary", "cache", ""}

	got := FindSimilarStrings("web-servr", candidates, 3)
	want := []string{"web-server", "web-server-2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("typo: got %v, want %v", got, want)
	}

	if got := FindSimilarStrings("db", candidates, 3); !reflect.DeepEqual(got, []string{"db-primary"}) {
		t.Errorf("substring: got %v", got)
	}
	if got := FindSimilarStrings("zzzzzzzzzz", candidates, 3); len(got) != 0 {
		t.Errorf("unrelated: got %v", got)
	}
	if got := FindSimilarStrings("web", candidates, 1); len(got) != 1 {
		t.Errorf("maxResults: got %v", got)
	}
	if got := FindSimilarStrings("x", nil, 3); got != nil {
		t.Errorf("no candidates: got %v", got)
	}
}
