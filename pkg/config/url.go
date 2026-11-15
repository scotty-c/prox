package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// NormalizeURL tidies a Proxmox URL before it is saved, so that the usual ways
// of writing one all work:
//
//   - a missing scheme means https (Proxmox serves https)
//   - a trailing slash, and a trailing /api2/json, are dropped
//   - a query or fragment is dropped
//
// It rejects an empty URL, one with no host, and schemes other than https and
// http. It does not add a port: a Proxmox node listens on 8006, but one behind
// a reverse proxy does not, so a URL with no port comes back with a note
// saying so rather than a guess.
func NormalizeURL(raw string) (normalized, note string, err error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", "", errors.New("the URL is empty")
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Hostname() == "" {
		return "", "", fmt.Errorf("%q is not a valid URL (expected something like https://pve.example.com:8006)", raw)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", "", fmt.Errorf("the URL must start with https:// (or http://), not %s://", u.Scheme)
	}

	path := strings.TrimRight(u.Path, "/")
	path = strings.TrimSuffix(path, "/api2/json")
	u.Path, u.RawPath, u.RawQuery, u.Fragment = strings.TrimRight(path, "/"), "", "", ""

	if u.Port() == "" {
		note = fmt.Sprintf("no port in %s: Proxmox listens on 8006 by default, so you probably want %s:8006 (ignore this if you reach it through a reverse proxy)", u.Host, u.Scheme+"://"+u.Host)
	}
	return u.String(), note, nil
}
