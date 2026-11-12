// Package sshconfig edits OpenSSH client config files (~/.ssh/config).
//
// It works line by line so that everything it does not touch (comments,
// formatting, Include and Match blocks) is preserved exactly.
package sshconfig

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Entry is a Host block as shown by `prox ssh --list`
type Entry struct {
	Hosts    []string
	HostName string
	User     string
	Port     string
	Identity string
}

// parseDirective splits a config line into its keyword and value.
// ssh_config allows "Keyword value" and "Keyword=value". Blank lines and
// comments return an empty keyword.
func parseDirective(line string) (keyword, value string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", ""
	}
	end := strings.IndexAny(trimmed, " \t=")
	if end == -1 {
		return strings.ToLower(trimmed), ""
	}
	keyword = strings.ToLower(trimmed[:end])
	value = strings.TrimSpace(trimmed[end:])
	value = strings.TrimSpace(strings.TrimPrefix(value, "="))
	return keyword, value
}

// isBlockStart reports whether a keyword starts a new Host or Match block
func isBlockStart(keyword string) bool {
	return keyword == "host" || keyword == "match"
}

// RemoveHost removes Host blocks for alias from content.
//
// If exact is true, only blocks whose Host line names exactly that one alias
// are removed (used when replacing an entry, so blocks shared with other
// aliases are left alone). If exact is false, any block whose Host line
// includes alias is removed.
//
// A block ends at the next Host or Match line. If the block's directives are
// indented, anything after the last indented directive (comments and
// unindented lines such as a top-level Include) is kept, because it is almost
// certainly not part of the block. Older prox versions inserted entries at the
// top of the file, in front of the global section, so this also repairs those
// files. It returns the new content, the removed text and whether anything
// was removed.
func RemoveHost(content, alias string, exact bool) (string, string, bool) {
	lines := strings.Split(content, "\n")
	var out, removed strings.Builder
	var block []string
	inTarget := false
	found := false

	flushBlock := func() {
		keepFrom := blockEnd(block)
		for _, l := range block[:keepFrom] {
			removed.WriteString(l)
			removed.WriteString("\n")
		}
		for _, l := range block[keepFrom:] {
			out.WriteString(l)
			out.WriteString("\n")
		}
		block = nil
	}

	for i, line := range lines {
		keyword, value := parseDirective(line)
		if isBlockStart(keyword) {
			if inTarget {
				flushBlock()
				inTarget = false
			}
			if keyword == "host" && hostMatches(value, alias, exact) {
				inTarget = true
				found = true
			}
		}

		if inTarget {
			block = append(block, line)
			continue
		}

		// Don't add a newline after the final line; strings.Split leaves an
		// empty last element when content ends with a newline.
		if i < len(lines)-1 {
			out.WriteString(line)
			out.WriteString("\n")
		} else if line != "" {
			out.WriteString(line)
		}
	}
	if inTarget {
		flushBlock()
	}

	result := out.String()
	if found && strings.HasSuffix(content, "\n") && !strings.HasSuffix(result, "\n") && result != "" {
		result += "\n"
	}
	return result, removed.String(), found
}

// blockEnd returns how many lines at the start of block belong to the block.
// The rest are kept in the file.
func blockEnd(block []string) int {
	lastIndented := 0
	for i, l := range block[1:] {
		keyword, _ := parseDirective(l)
		if keyword != "" && (strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t")) {
			lastIndented = i + 1
		}
	}

	end := len(block)
	if lastIndented > 0 {
		// Indented style: the block ends after its last indented directive
		end = lastIndented + 1
		// Remove the blank separator lines that follow it as well
		for end < len(block) && strings.TrimSpace(block[end]) == "" {
			end++
		}
	} else {
		// Unindented style: keep only trailing comments, which usually
		// describe the next block
		for end > 1 && strings.HasPrefix(strings.TrimSpace(block[end-1]), "#") {
			end--
		}
	}
	return end
}

func hostMatches(value, alias string, exact bool) bool {
	hosts := strings.Fields(value)
	if exact {
		return len(hosts) == 1 && hosts[0] == alias
	}
	return slices.Contains(hosts, alias)
}

// AddHost replaces any existing single-alias block for alias with entry.
//
// The entry is inserted after the global section at the top of the file
// (directives before the first Host or Match line apply to every host, so
// putting a Host block in front of them would change their meaning) and
// before the first existing block, so it takes precedence over wildcard
// blocks such as "Host *".
func AddHost(content, alias, entry string) string {
	rest, _, _ := RemoveHost(content, alias, true)
	if !strings.HasSuffix(entry, "\n") {
		entry += "\n"
	}

	lines := strings.Split(rest, "\n")
	insertAt := -1
	for i, line := range lines {
		keyword, _ := parseDirective(line)
		if isBlockStart(keyword) {
			insertAt = i
			break
		}
	}

	if insertAt == -1 {
		// No blocks: append after the global section
		if rest == "" {
			return entry
		}
		if !strings.HasSuffix(rest, "\n") {
			rest += "\n"
		}
		if !strings.HasSuffix(rest, "\n\n") {
			rest += "\n"
		}
		return rest + entry
	}

	// Keep comments that directly precede the first block with that block
	for insertAt > 0 && strings.HasPrefix(strings.TrimSpace(lines[insertAt-1]), "#") {
		insertAt--
	}

	before := strings.Join(lines[:insertAt], "\n")
	after := strings.Join(lines[insertAt:], "\n")
	if before != "" {
		before += "\n"
	}
	return before + entry + "\n" + after
}

// ParseEntries returns the Host blocks in content
func ParseEntries(content string) []Entry {
	var entries []Entry
	var current *Entry

	flush := func() {
		if current != nil {
			entries = append(entries, *current)
			current = nil
		}
	}

	for _, line := range strings.Split(content, "\n") {
		keyword, value := parseDirective(line)
		switch {
		case keyword == "":
			continue
		case keyword == "host":
			flush()
			if fields := strings.Fields(value); len(fields) > 0 {
				current = &Entry{Hosts: fields, Port: "22"}
			}
		case keyword == "match":
			flush()
		case current == nil:
			continue
		case keyword == "hostname":
			current.HostName = value
		case keyword == "user":
			current.User = value
		case keyword == "port":
			current.Port = value
		case keyword == "identityfile":
			current.Identity = value
		}
	}
	flush()
	return entries
}

// WriteFile writes data to path without leaving a partly written file if it
// fails: it writes a temporary file in the same directory and renames it into
// place. If path is a symlink (common with dotfile managers) the link target
// is updated and the link is kept. Before replacing an existing file, a copy
// is saved alongside it as <name>.prox.bak.
func WriteFile(path string, data []byte) error {
	target := path
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		target = resolved
	}

	mode := os.FileMode(0600)
	if info, err := os.Stat(target); err == nil {
		mode = info.Mode().Perm()
		if err := copyFile(target, target+".prox.bak", mode); err != nil {
			return fmt.Errorf("failed to back up %s: %w", target, err)
		}
	}

	tmp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("failed to write temporary file: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("failed to set permissions: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("failed to sync temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("failed to close temporary file: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		cleanup()
		return fmt.Errorf("failed to replace %s: %w", target, err)
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
