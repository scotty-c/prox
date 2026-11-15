package config

import (
	"strings"
	"testing"
)

func TestParseProfile(t *testing.T) {
	input := strings.Join([]string{
		"# a comment",
		"",
		"username=abc==",        // base64 padding: '=' inside a value
		"  password = secret  ", // spaces around key and value
		"url=https://pve:8006/?a=b",
		"insecure=true",
		"not a setting",
		"url=https://override:8006", // the last one wins
		fingerprintComment + " 0123abcd",
		"# trailing comment",
	}, "\n")
	values, fingerprint, err := parseProfile(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"username": "abc==", "password": "secret", "url": "https://override:8006", "insecure": "true",
	}
	if len(values) != len(want) {
		t.Errorf("values = %v, want %v", values, want)
	}
	for k, v := range want {
		if values[k] != v {
			t.Errorf("values[%q] = %q, want %q", k, values[k], v)
		}
	}
	if fingerprint != "0123abcd" {
		t.Errorf("fingerprint = %q", fingerprint)
	}
}

func TestParseProfileWithNothingInIt(t *testing.T) {
	values, fingerprint, err := parseProfile(strings.NewReader("\n# only a comment\n"))
	if err != nil || len(values) != 0 || fingerprint != "" {
		t.Errorf("%v %q %v", values, fingerprint, err)
	}
}

func TestReadProfileFileReportsAMissingFile(t *testing.T) {
	if _, _, err := readProfileFile(t.TempDir() + "/nope"); err == nil {
		t.Error("expected an error for a missing file")
	}
}
