package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// parseProfile reads the key=value lines of a profile file. Blank lines and
// comments are skipped, except for the comment that records the fingerprint of
// the key the credentials were encrypted with, which is returned separately
// ("" if there is none). Only the first '=' on a line separates the key from
// the value, so a value may contain '='. If a key appears twice, the last wins.
func parseProfile(r io.Reader) (values map[string]string, fingerprint string, err error) {
	values = map[string]string{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if fp, ok := strings.CutPrefix(line, fingerprintComment); ok {
			fingerprint = strings.TrimSpace(fp)
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	if err := scanner.Err(); err != nil {
		return nil, "", err
	}
	return values, fingerprint, nil
}

// readProfileFile opens and parses the profile file at path
func readProfileFile(path string) (map[string]string, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()
	values, fingerprint, err := parseProfile(file)
	if err != nil {
		return nil, "", fmt.Errorf("error reading %s: %w", path, err)
	}
	return values, fingerprint, nil
}
