// Package fakepve is a small fake of the Proxmox VE API for testing prox end
// to end. It serves a fixed cluster (two nodes, four VMs, two containers),
// records every request, and mirrors real Proxmox behaviour where prox has
// been wrong about it before: the task log returns 50 lines unless asked
// for more, VM config values such as agent and memory are strings, and LXC
// has no guest agent endpoint.
package fakepve

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// UPID is the task ID every operation returns
const UPID = "UPID:pve1:0000AAAA:0000BBBB:65000000:qmstart:100:root@pam:"

// LogLines is the length of every task log
const LogLines = 120

// Server is a running fake Proxmox API
type Server struct {
	*httptest.Server

	mu       sync.Mutex
	requests []string
	failures map[string]string
}

// New starts a fake Proxmox API that is shut down when the test ends
func New(t testing.TB) *Server {
	t.Helper()
	s := &Server{failures: map[string]string{}}
	s.Server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.Close)
	return s
}

// FailRequest makes requests to "METHOD /path" (the path without /api2/json)
// fail with HTTP 500 and msg as the reason
func (s *Server) FailRequest(methodAndPath, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures[methodAndPath] = msg
}

// Requests returns every request received so far as "METHOD /path?query"
func (s *Server) Requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.requests...)
}

// Count returns how many requests started with method and path prefix
func (s *Server) Count(method, pathPrefix string) int {
	n := 0
	for _, r := range s.Requests() {
		if strings.HasPrefix(r, method+" "+pathPrefix) {
			n++
		}
	}
	return n
}

var cluster = []map[string]interface{}{
	{"id": "node/pve1", "type": "node", "node": "pve1", "status": "online", "cpu": 0.1, "maxcpu": 8, "mem": 8 << 30, "maxmem": 32 << 30, "uptime": 99999},
	{"id": "node/pve2", "type": "node", "node": "pve2", "status": "online", "cpu": 0.2, "maxcpu": 8, "mem": 4 << 30, "maxmem": 32 << 30, "uptime": 99999},
	{"id": "qemu/1000", "type": "qemu", "node": "pve2", "vmid": 1000, "name": "big", "status": "running", "cpu": 0.5, "maxcpu": 4, "mem": 1 << 30, "maxmem": 4 << 30, "maxdisk": 64 << 30, "disk": 0, "uptime": 3600},
	{"id": "qemu/100", "type": "qemu", "node": "pve1", "vmid": 100, "name": "web", "status": "running", "cpu": 0.02, "maxcpu": 2, "mem": 1 << 30, "maxmem": 2 << 30, "maxdisk": 32 << 30, "disk": 0, "uptime": 86400 * 3, "tags": "prod;web"},
	{"id": "qemu/101", "type": "qemu", "node": "pve1", "vmid": 101, "name": "db", "status": "stopped", "maxcpu": 2, "maxmem": 2 << 30, "maxdisk": 32 << 30, "tags": "prod"},
	{"id": "qemu/102", "type": "qemu", "node": "pve2", "vmid": 102, "name": "web", "status": "stopped", "maxcpu": 2, "maxmem": 2 << 30, "maxdisk": 32 << 30},
	{"id": "lxc/200", "type": "lxc", "node": "pve1", "vmid": 200, "name": "ct1", "status": "running", "cpu": 0.01, "maxcpu": 1, "mem": 100 << 20, "maxmem": 512 << 20, "maxdisk": 8 << 30, "disk": 1 << 30, "uptime": 500},
	{"id": "lxc/201", "type": "lxc", "node": "pve1", "vmid": 201, "name": "ct2", "status": "stopped", "maxcpu": 1, "maxmem": 512 << 20, "maxdisk": 8 << 30},
}

var qemuConfig = map[string]interface{}{
	"name": "web", "agent": "1", "memory": "2048", "cores": 2, "sockets": 1,
	"scsi0": "local-lvm:vm-100-disk-0,size=32G", "scsi5": "local-lvm:vm-100-disk-1,size=100G",
	"ide2": "local-lvm:vm-100-cloudinit,media=cdrom", "net0": "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0",
	"boot": "order=scsi0", "onboot": 1,
}

var (
	reQemuConfig = regexp.MustCompile(`^/nodes/\w+/qemu/\d+/config$`)
	reQemuStatus = regexp.MustCompile(`^/nodes/\w+/qemu/\d+/status/current$`)
	reLxcStatus  = regexp.MustCompile(`^/nodes/\w+/lxc/\d+/status/current$`)
	reLxcIfaces  = regexp.MustCompile(`^/nodes/\w+/lxc/\d+/interfaces$`)
	reLxcConfig  = regexp.MustCompile(`^/nodes/\w+/lxc/\d+/config$`)
	reAction     = regexp.MustCompile(`^/nodes/\w+/(qemu|lxc)/(\d+)/status/\w+$`)
	reGuest      = regexp.MustCompile(`^/nodes/\w+/(qemu|lxc)/\d+$`)
	reSnapshots  = regexp.MustCompile(`^/nodes/\w+/(qemu|lxc)/\d+/snapshot$`)
)

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path, "/api2/json")
	s.mu.Lock()
	s.requests = append(s.requests, r.Method+" "+strings.TrimPrefix(r.URL.RequestURI(), "/api2/json"))
	msg, failed := s.failures[r.Method+" "+p]
	s.mu.Unlock()
	if failed {
		reply(w, nil, http.StatusInternalServerError, msg)
		return
	}

	switch {
	case p == "/access/ticket":
		reply(w, map[string]interface{}{"ticket": "T", "CSRFPreventionToken": "C", "username": "root@pam"}, 200, "")
	case p == "/version":
		reply(w, map[string]interface{}{"version": "8.2.4", "release": "8.2", "repoid": "abc"}, 200, "")
	case p == "/nodes":
		reply(w, []map[string]interface{}{
			{"node": "pve1", "status": "online", "type": "node", "id": "node/pve1"},
			{"node": "pve2", "status": "online", "type": "node", "id": "node/pve2"}}, 200, "")
	case p == "/cluster/resources":
		reply(w, cluster, 200, "")
	case p == "/cluster/nextid":
		reply(w, "103", 200, "")
	case reQemuConfig.MatchString(p):
		if r.Method == http.MethodGet {
			reply(w, qemuConfig, 200, "")
		} else {
			reply(w, nil, 200, "")
		}
	case reQemuStatus.MatchString(p):
		reply(w, map[string]interface{}{"status": "running", "uptime": 1000, "cpu": 0.02, "mem": 1 << 30, "maxmem": 2 << 30, "maxdisk": 32 << 30, "disk": 0}, 200, "")
	case reLxcStatus.MatchString(p):
		reply(w, map[string]interface{}{"status": "running", "uptime": 500, "cpu": 0.01, "mem": 1 << 20, "maxmem": 512 << 20}, 200, "")
	case reLxcIfaces.MatchString(p):
		reply(w, []map[string]interface{}{{"name": "lo", "inet": "127.0.0.1/8"}, {"name": "eth0", "inet": "10.0.0.50/24"}}, 200, "")
	case reLxcConfig.MatchString(p):
		reply(w, map[string]interface{}{"hostname": "ct1", "memory": 512, "cores": 1, "rootfs": "local-lvm:vm-200-disk-0,size=8G", "net0": "name=eth0,bridge=vmbr0,ip=dhcp"}, 200, "")
	case strings.Contains(p, "/agent/"):
		reply(w, nil, 500, "QEMU guest agent is not running")
	case strings.HasSuffix(p, "/resize") && r.Method == http.MethodPut:
		reply(w, UPID, 200, "")
	case reSnapshots.MatchString(p) && r.Method == http.MethodGet:
		reply(w, []map[string]interface{}{{"name": "snap1", "snaptime": 1700000000, "description": "before"}, {"name": "current"}}, 200, "")
	case strings.Contains(p, "/snapshot/"):
		reply(w, UPID, 200, "")
	case reAction.MatchString(p) && r.Method == http.MethodPost:
		reply(w, UPID, 200, "")
	case reGuest.MatchString(p) && r.Method == http.MethodDelete:
		reply(w, UPID, 200, "")
	case strings.HasPrefix(p, "/nodes/") && strings.HasSuffix(p, "/status") && strings.Contains(p, "/tasks/"):
		reply(w, map[string]interface{}{"upid": UPID, "node": "pve1", "status": "stopped", "exitstatus": "OK", "type": "qmstart", "pid": 1}, 200, "")
	case strings.HasPrefix(p, "/nodes/") && strings.HasSuffix(p, "/log") && strings.Contains(p, "/tasks/"):
		start, limit := 0, 50 // Proxmox returns 50 lines unless asked for more
		if v := r.URL.Query().Get("start"); v != "" {
			start, _ = strconv.Atoi(v)
		}
		if v := r.URL.Query().Get("limit"); v != "" {
			limit, _ = strconv.Atoi(v)
		}
		var lines []map[string]interface{}
		for n := start; n < LogLines && n < start+limit; n++ {
			lines = append(lines, map[string]interface{}{"n": n + 1, "t": fmt.Sprintf("log line %d", n+1)})
		}
		reply(w, lines, 200, "")
	default:
		reply(w, nil, http.StatusNotImplemented, "not implemented in fakepve: "+r.Method+" "+p)
	}
}

func reply(w http.ResponseWriter, data interface{}, status int, reason string) {
	payload := map[string]interface{}{"data": data}
	if reason != "" {
		// Real Proxmox puts the reason in the HTTP status line, which Go's
		// server cannot set; prox reads the body's message field too.
		payload["message"] = reason
	}
	body, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
