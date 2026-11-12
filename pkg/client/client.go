package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/scotty-c/prox/pkg/config"
)

// ProxmoxClient represents a client for Proxmox VE API
type ProxmoxClient struct {
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
	ticket     string
	csrfToken  string
	authMu     sync.Mutex // guards ticket and csrfToken

	// Cluster resources cache
	cachedResources      []Resource
	cachedResourcesTime  time.Time
	cachedResourcesMutex sync.RWMutex

	// IP address cache for VMs and containers
	ipCache      map[string]ipCacheEntry
	ipCacheMutex sync.RWMutex
}

// ipCacheEntry represents a cached IP address with its timestamp
type ipCacheEntry struct {
	ip        string
	timestamp time.Time
}

// AuthResponse represents the response from Proxmox authentication
type AuthResponse struct {
	Data struct {
		Ticket              string                 `json:"ticket"`
		CSRFPreventionToken string                 `json:"CSRFPreventionToken"`
		Username            string                 `json:"username"`
		Cap                 map[string]interface{} `json:"cap"`
	} `json:"data"`
}

// APIResponse represents a generic API response
type APIResponse struct {
	Data   interface{} `json:"data"`
	Errors interface{} `json:"errors"`
}

// ClusterResourcesResponse represents the response from cluster resources endpoint
type ClusterResourcesResponse struct {
	Data   []Resource  `json:"data"`
	Errors interface{} `json:"errors"`
}

// Node represents a Proxmox node
type Node struct {
	Node   string `json:"node"`
	Status string `json:"status"`
	Type   string `json:"type"`
	ID     string `json:"id"`
}

// Resource represents a cluster resource
type Resource struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Node    string   `json:"node"`
	VMID    *int     `json:"vmid,omitempty"`
	Name    string   `json:"name,omitempty"`
	Status  string   `json:"status,omitempty"`
	MaxCPU  *int     `json:"maxcpu,omitempty"`
	CPU     *float64 `json:"cpu,omitempty"`
	MaxMem  *int64   `json:"maxmem,omitempty"`
	Mem     *int64   `json:"mem,omitempty"`
	MaxDisk *int64   `json:"maxdisk,omitempty"`
	Disk    *int64   `json:"disk,omitempty"`
	Uptime  *int64   `json:"uptime,omitempty"`
	Tags    string   `json:"tags,omitempty"` // Semicolon-separated tags
}

// Version represents Proxmox version info
type Version struct {
	Version string `json:"version"`
	Release string `json:"release"`
	RepoID  string `json:"repoid"`
}

// Task represents a Proxmox task
type Task struct {
	UPID     string `json:"upid"`
	Node     string `json:"node"`
	PID      int    `json:"pid"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	ExitCode string `json:"exitstatus,omitempty"`
}

// ContainerTemplate represents a container template
type ContainerTemplate struct {
	VolID string `json:"volid"`
	Size  uint64 `json:"size"`
	Used  uint64 `json:"used,omitempty"`
}

// ProxmoxClientInterface defines the interface for Proxmox client operations
// This interface enables mocking and testing
type ProxmoxClientInterface interface {
	// Authentication
	Authenticate(ctx context.Context) error

	// Version and cluster info
	GetVersion(ctx context.Context) (*Version, error)
	GetNodes(ctx context.Context) ([]Node, error)
	GetClusterResources(ctx context.Context) ([]Resource, error)
	GetClusterStatus(ctx context.Context) (*ClusterStatus, error)
	ClearClusterResourcesCache()

	// VM operations
	StartVM(ctx context.Context, node string, vmid int) (string, error)
	StopVM(ctx context.Context, node string, vmid int) (string, error)
	ForceStopVM(ctx context.Context, node string, vmid int) (string, error)
	DeleteVM(ctx context.Context, node string, vmid int) (string, error)
	CloneVM(ctx context.Context, node string, vmid int, newid int, name string, full bool) (string, error)
	UpdateVM(ctx context.Context, node string, vmid int, config map[string]interface{}) (string, error)
	ResizeDisk(ctx context.Context, node string, vmid int, disk string, size string) (string, error)
	GetVMConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error)
	GetVMStatus(ctx context.Context, node string, vmid int) (map[string]interface{}, error)
	GetVMDiskInfo(ctx context.Context, node string, vmid int) (uint64, uint64, error)
	GetVMNode(ctx context.Context, vmid int) (string, error)
	GetVMIP(ctx context.Context, node string, vmid int) (string, error)
	GetVMNetworkInterfaces(ctx context.Context, node string, vmid int) ([]NetworkInterface, error)
	IsVMRunning(ctx context.Context, vmid int) (bool, error)
	MigrateVM(ctx context.Context, node string, vmid int, targetNode string, options map[string]interface{}) (string, error)

	// Snapshot operations (guestType is GuestVM or GuestContainer)
	ListSnapshots(ctx context.Context, node, guestType string, vmid int) ([]Snapshot, error)
	CreateSnapshot(ctx context.Context, node, guestType string, vmid int, name, description string, vmState bool) (string, error)
	RollbackSnapshot(ctx context.Context, node, guestType string, vmid int, name string) (string, error)
	DeleteSnapshot(ctx context.Context, node, guestType string, vmid int, name string) (string, error)

	// Container operations
	CreateContainer(ctx context.Context, node string, vmid int, params map[string]interface{}) (string, error)
	StartContainer(ctx context.Context, node string, vmid int) (string, error)
	StopContainer(ctx context.Context, node string, vmid int) (string, error)
	ForceStopContainer(ctx context.Context, node string, vmid int) (string, error)
	DeleteContainer(ctx context.Context, node string, vmid int) (string, error)
	GetContainerConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error)
	GetContainerStatus(ctx context.Context, node string, vmid int) (map[string]interface{}, error)
	GetContainerTemplates(ctx context.Context, node string) ([]ContainerTemplate, error)
	GetStorageNames(ctx context.Context, node string, content string) ([]string, error)
	GetContainerIP(ctx context.Context, node string, vmid int) (string, error)
	GetContainerIPAlternative(ctx context.Context, node string, vmid int) (string, error)
	GetContainerNetworkInterfaces(ctx context.Context, node string, vmid int) ([]NetworkInterface, error)
	IsContainerRunning(ctx context.Context, vmid int) (bool, error)

	// Node operations
	GetNodeIP(ctx context.Context, node string) (string, error)

	// Task operations
	GetNextVMID(ctx context.Context) (int, error)
	GetTaskStatus(ctx context.Context, node, upid string) (*Task, error)
	GetTaskLog(ctx context.Context, node, upid string, start int, limit int) ([]string, error)
	WaitForTask(ctx context.Context, node, taskID string) (*Task, error)
}

// Client cache for reuse across operations
var (
	cachedClient     *ProxmoxClient
	cachedClientKey  string
	clientCacheMutex sync.RWMutex
)

// NewClient creates a new Proxmox client that verifies the server certificate
// against the system CAs. Use NewClientWithTLS to trust another CA or to skip
// verification.
func NewClient(baseURL, username, password string) *ProxmoxClient {
	client, _ := NewClientWithTLS(baseURL, username, password, TLSOptions{})
	return client
}

// NewClientWithTLS creates a new Proxmox client with the given TLS options
func NewClientWithTLS(baseURL, username, password string, opts TLSOptions) (*ProxmoxClient, error) {
	tlsConfig, err := buildTLSConfig(opts)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
		// Connection pooling settings for better reuse
		MaxIdleConns:        100,              // Maximum idle connections across all hosts
		MaxIdleConnsPerHost: 10,               // Maximum idle connections per host
		MaxConnsPerHost:     10,               // Maximum total connections per host
		IdleConnTimeout:     90 * time.Second, // How long idle connections stay alive
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   DefaultTimeout * time.Second,
	}

	return &ProxmoxClient{
		BaseURL:    strings.TrimSuffix(baseURL, "/"),
		Username:   username,
		Password:   password,
		HTTPClient: client,
		ipCache:    make(map[string]ipCacheEntry),
	}, nil
}

// IsAPIToken reports whether username is a Proxmox API token ID
// (user@realm!tokenname). Normal usernames cannot contain '!'.
func IsAPIToken(username string) bool {
	return strings.Contains(username, "!")
}

// usesAPIToken reports whether this client authenticates with an API token
func (c *ProxmoxClient) usesAPIToken() bool {
	return IsAPIToken(c.Username)
}

// Authenticate authenticates with Proxmox and stores the ticket.
// With an API token there is no ticket; the token is checked by calling the
// version endpoint instead.
func (c *ProxmoxClient) Authenticate(ctx context.Context) error {
	if c.usesAPIToken() {
		if _, err := c.GetVersion(ctx); err != nil {
			return fmt.Errorf("authentication failed: API token rejected: %w", err)
		}
		return nil
	}

	c.authMu.Lock()
	defer c.authMu.Unlock()
	return c.authenticateLocked(ctx)
}

// authenticateLocked logs in with username and password. c.authMu must be held.
func (c *ProxmoxClient) authenticateLocked(ctx context.Context) error {
	authURL := fmt.Sprintf("%s/api2/json/access/ticket", c.BaseURL)

	data := url.Values{}
	data.Set("username", c.Username)
	data.Set("password", c.Password)

	req, err := http.NewRequestWithContext(ctx, "POST", authURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create auth request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return wrapTLSError(fmt.Errorf("authentication request failed: %w", err), c.BaseURL)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read auth response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed with status %d: %s", resp.StatusCode, string(body))
	}

	var authResp AuthResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		return fmt.Errorf("failed to parse auth response: %w", err)
	}

	c.ticket = authResp.Data.Ticket
	c.csrfToken = authResp.Data.CSRFPreventionToken

	return nil
}

// makeRequest makes an authenticated request to the Proxmox API
func (c *ProxmoxClient) makeRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var jsonBody []byte
	if body != nil {
		var err error
		jsonBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	for attempt := 0; ; attempt++ {
		ticket, csrfToken, err := c.ensureTicket(ctx)
		if err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}

		status, reason, respBody, err := c.doRequest(ctx, method, path, jsonBody, ticket, csrfToken)
		if err != nil {
			return nil, err
		}

		// Tickets expire after two hours. Log in again once and retry.
		if status == http.StatusUnauthorized && !c.usesAPIToken() && attempt == 0 {
			c.clearTicket(ticket)
			continue
		}

		if status < 200 || status >= 300 {
			return nil, fmt.Errorf("API request failed with status %d: %s", status, apiErrorMessage(reason, respBody))
		}
		return respBody, nil
	}
}

// doRequest sends one request and returns the status code, status reason and body
func (c *ProxmoxClient) doRequest(ctx context.Context, method, path string, jsonBody []byte, ticket, csrfToken string) (int, string, []byte, error) {
	reqURL := fmt.Sprintf("%s/api2/json%s", c.BaseURL, path)

	var reqBody io.Reader
	if jsonBody != nil {
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
	if err != nil {
		return 0, "", nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication headers
	if c.usesAPIToken() {
		// Token requests don't need a ticket or CSRF token
		req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.Username, c.Password))
	} else {
		req.Header.Set("Cookie", fmt.Sprintf("PVEAuthCookie=%s", ticket))
		if method != "GET" && method != "HEAD" {
			req.Header.Set("CSRFPreventionToken", csrfToken)
		}
	}

	if jsonBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, "", nil, wrapTLSError(fmt.Errorf("request failed: %w", err), c.BaseURL)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", nil, fmt.Errorf("failed to read response: %w", err)
	}

	// resp.Status is e.g. "500 VM 100 not running"; Proxmox puts the reason there
	reason := strings.TrimSpace(strings.TrimPrefix(resp.Status, strconv.Itoa(resp.StatusCode)))
	return resp.StatusCode, reason, respBody, nil
}

// ensureTicket logs in if there is no ticket yet and returns the current
// ticket and CSRF token. API token clients return empty values.
func (c *ProxmoxClient) ensureTicket(ctx context.Context) (string, string, error) {
	if c.usesAPIToken() {
		return "", "", nil
	}
	c.authMu.Lock()
	defer c.authMu.Unlock()
	if c.ticket == "" {
		if err := c.authenticateLocked(ctx); err != nil {
			return "", "", err
		}
	}
	return c.ticket, c.csrfToken, nil
}

// clearTicket forgets a ticket that the server rejected, unless another
// request has already replaced it
func (c *ProxmoxClient) clearTicket(rejected string) {
	c.authMu.Lock()
	defer c.authMu.Unlock()
	if c.ticket == rejected {
		c.ticket = ""
		c.csrfToken = ""
	}
}

// maxErrorBodyLength limits how much of an unrecognised error body is shown
const maxErrorBodyLength = 300

// apiErrorMessage builds a readable message from a failed API response
func apiErrorMessage(reason string, body []byte) string {
	var parsed struct {
		Message string            `json:"message"`
		Errors  map[string]string `json:"errors"`
	}
	var parts []string
	if reason != "" {
		parts = append(parts, reason)
	}
	if err := json.Unmarshal(body, &parsed); err == nil {
		if msg := strings.TrimSpace(parsed.Message); msg != "" && msg != reason {
			parts = append(parts, msg)
		}
		if len(parsed.Errors) > 0 {
			keys := make([]string, 0, len(parsed.Errors))
			for k := range parsed.Errors {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			var errs []string
			for _, k := range keys {
				errs = append(errs, fmt.Sprintf("%s: %s", k, strings.TrimSpace(parsed.Errors[k])))
			}
			parts = append(parts, "("+strings.Join(errs, "; ")+")")
		}
	} else if text := strings.TrimSpace(string(body)); text != "" {
		if len(text) > maxErrorBodyLength {
			text = text[:maxErrorBodyLength] + "..."
		}
		parts = append(parts, text)
	}
	if len(parts) == 0 {
		return "no details returned"
	}
	return strings.Join(parts, " ")
}

// GetVersion gets the Proxmox version
func (c *ProxmoxClient) GetVersion(ctx context.Context) (*Version, error) {
	body, err := c.makeRequest(ctx, "GET", "/version", nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse version response: %w", err)
	}

	versionData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected version response format")
	}

	version := &Version{}
	if v, ok := versionData["version"].(string); ok {
		version.Version = v
	}
	if r, ok := versionData["release"].(string); ok {
		version.Release = r
	}
	if repo, ok := versionData["repoid"].(string); ok {
		version.RepoID = repo
	}

	return version, nil
}

// GetNodes gets the list of cluster nodes
func (c *ProxmoxClient) GetNodes(ctx context.Context) ([]Node, error) {
	body, err := c.makeRequest(ctx, "GET", "/nodes", nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse nodes response: %w", err)
	}

	nodesData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected nodes response format")
	}

	var nodes []Node
	for _, nodeData := range nodesData {
		nodeMap, ok := nodeData.(map[string]interface{})
		if !ok {
			continue
		}

		node := Node{}
		if v, ok := nodeMap["node"].(string); ok {
			node.Node = v
		}
		if v, ok := nodeMap["status"].(string); ok {
			node.Status = v
		}
		if v, ok := nodeMap["type"].(string); ok {
			node.Type = v
		}
		if v, ok := nodeMap["id"].(string); ok {
			node.ID = v
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}

// GetClusterResources gets cluster resources with short-lived caching
func (c *ProxmoxClient) GetClusterResources(ctx context.Context) ([]Resource, error) {
	// Try to use cached data (read lock)
	c.cachedResourcesMutex.RLock()
	if time.Since(c.cachedResourcesTime) < time.Duration(ClusterResourcesCacheTTL)*time.Second {
		cached := c.cachedResources
		c.cachedResourcesMutex.RUnlock()
		return cached, nil
	}
	c.cachedResourcesMutex.RUnlock()

	// Cache miss or expired, fetch from API (write lock)
	c.cachedResourcesMutex.Lock()
	defer c.cachedResourcesMutex.Unlock()

	// Double-check after acquiring write lock (another goroutine might have refreshed)
	if time.Since(c.cachedResourcesTime) < time.Duration(ClusterResourcesCacheTTL)*time.Second {
		return c.cachedResources, nil
	}

	// Fetch from API
	body, err := c.makeRequest(ctx, "GET", "/cluster/resources", nil)
	if err != nil {
		return nil, err
	}

	var resp ClusterResourcesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse cluster resources response: %w", err)
	}

	// Update cache
	c.cachedResources = resp.Data
	c.cachedResourcesTime = time.Now()

	return resp.Data, nil
}

// StartVM starts a virtual machine
func (c *ProxmoxClient) StartVM(ctx context.Context, node string, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/start", url.PathEscape(node), vmid)
	return requestData[string](ctx, c, "POST", path, nil, "start VM")
}

// StopVM stops a virtual machine
func (c *ProxmoxClient) StopVM(ctx context.Context, node string, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/shutdown", url.PathEscape(node), vmid)
	return requestData[string](ctx, c, "POST", path, nil, "stop VM")
}

// ForceStopVM stops a virtual machine immediately, like pulling the power.
// Use StopVM for a clean ACPI shutdown.
func (c *ProxmoxClient) ForceStopVM(ctx context.Context, node string, vmid int) (string, error) {
	return requestData[string](ctx, c, "POST", fmt.Sprintf("/nodes/%s/qemu/%d/status/stop", url.PathEscape(node), vmid), nil, "force stop VM")
}

// ForceStopContainer stops a container immediately. Use StopContainer for a
// clean shutdown.
func (c *ProxmoxClient) ForceStopContainer(ctx context.Context, node string, vmid int) (string, error) {
	return requestData[string](ctx, c, "POST", fmt.Sprintf("/nodes/%s/lxc/%d/status/stop", url.PathEscape(node), vmid), nil, "force stop container")
}

// requestData makes an API request and returns its "data" field as T.
// Errors read "failed to parse <action> response" if the body isn't JSON and
// "unexpected <action> response format" if data isn't a T.
func requestData[T any](ctx context.Context, c *ProxmoxClient, method, path string, body interface{}, action string) (T, error) {
	var zero T
	raw, err := c.makeRequest(ctx, method, path, body)
	if err != nil {
		return zero, err
	}

	var resp APIResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return zero, fmt.Errorf("failed to parse %s response: %w", action, err)
	}

	data, ok := resp.Data.(T)
	if !ok {
		return zero, fmt.Errorf("unexpected %s response format", action)
	}
	return data, nil
}

// DeleteVM deletes a virtual machine
func (c *ProxmoxClient) DeleteVM(ctx context.Context, node string, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d", url.PathEscape(node), vmid)
	return requestData[string](ctx, c, "DELETE", path, nil, "delete VM")
}

// CloneVM clones a virtual machine
func (c *ProxmoxClient) CloneVM(ctx context.Context, node string, vmid int, newid int, name string, full bool) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/clone", url.PathEscape(node), vmid)
	reqBody := map[string]interface{}{
		"newid": newid,
		"name":  name,
	}

	// Add full parameter if requested to create a full clone instead of linked clone
	if full {
		reqBody["full"] = ProxmoxBoolTrue
	}

	return requestData[string](ctx, c, "POST", path, reqBody, "clone VM")
}

// CreateContainer creates a new LXC container
func (c *ProxmoxClient) CreateContainer(ctx context.Context, node string, vmid int, params map[string]interface{}) (string, error) {
	path := fmt.Sprintf("/nodes/%s/lxc", url.PathEscape(node))

	// Ensure vmid is included in the request
	params["vmid"] = vmid

	return requestData[string](ctx, c, "POST", path, params, "create container")
}

// StartContainer starts an LXC container
func (c *ProxmoxClient) StartContainer(ctx context.Context, node string, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/lxc/%d/status/start", url.PathEscape(node), vmid)
	return requestData[string](ctx, c, "POST", path, nil, "start container")
}

// StopContainer stops an LXC container
func (c *ProxmoxClient) StopContainer(ctx context.Context, node string, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/lxc/%d/status/shutdown", url.PathEscape(node), vmid)
	return requestData[string](ctx, c, "POST", path, nil, "stop container")
}

// DeleteContainer deletes a container
func (c *ProxmoxClient) DeleteContainer(ctx context.Context, node string, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/lxc/%d", url.PathEscape(node), vmid)
	return requestData[string](ctx, c, "DELETE", path, nil, "delete container")
}

// GetContainerConfig gets the configuration of an LXC container
func (c *ProxmoxClient) GetContainerConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	path := fmt.Sprintf("/nodes/%s/lxc/%d/config", url.PathEscape(node), vmid)
	return requestData[map[string]interface{}](ctx, c, "GET", path, nil, "container config")
}

// GetContainerStatus gets the status of an LXC container
func (c *ProxmoxClient) GetContainerStatus(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	path := fmt.Sprintf("/nodes/%s/lxc/%d/status/current", url.PathEscape(node), vmid)
	return requestData[map[string]interface{}](ctx, c, "GET", path, nil, "container status")
}

// GetNextVMID gets the next available VM/container ID
func (c *ProxmoxClient) GetNextVMID(ctx context.Context) (int, error) {
	body, err := c.makeRequest(ctx, "GET", "/cluster/nextid", nil)
	if err != nil {
		return 0, err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("failed to parse next ID response: %w", err)
	}

	// Try different response formats that Proxmox might return
	switch data := resp.Data.(type) {
	case float64:
		return int(data), nil
	case string:
		// Parse string to int
		if id, err := strconv.Atoi(data); err == nil {
			return id, nil
		}
		return 0, fmt.Errorf("invalid next ID format: %s", data)
	case int:
		return data, nil
	default:
		return 0, fmt.Errorf("unexpected next ID response format: %T", resp.Data)
	}
}

// GetTaskStatus gets the status of a Proxmox task
func (c *ProxmoxClient) GetTaskStatus(ctx context.Context, node, upid string) (*Task, error) {
	path := fmt.Sprintf("/nodes/%s/tasks/%s/status", url.PathEscape(node), url.PathEscape(upid))

	body, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse task status response: %w", err)
	}

	taskData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected task status response format")
	}

	task := &Task{}
	if v, ok := taskData["upid"].(string); ok {
		task.UPID = v
	}
	if v, ok := taskData["node"].(string); ok {
		task.Node = v
	}
	if v, ok := taskData["type"].(string); ok {
		task.Type = v
	}
	if v, ok := taskData["status"].(string); ok {
		task.Status = v
	}
	if v, ok := taskData["exitstatus"].(string); ok {
		task.ExitCode = v
	}
	if v, ok := taskData["pid"].(float64); ok {
		task.PID = int(v)
	}

	return task, nil
}

// GetTaskLog retrieves the log for a Proxmox task
func (c *ProxmoxClient) GetTaskLog(ctx context.Context, node, upid string, start int, limit int) ([]string, error) {
	path := fmt.Sprintf("/nodes/%s/tasks/%s/log", url.PathEscape(node), url.PathEscape(upid))

	// Add query parameters for pagination
	params := url.Values{}
	if start > 0 {
		params.Add("start", strconv.Itoa(start))
	}
	if limit > 0 {
		params.Add("limit", strconv.Itoa(limit))
	}

	if len(params) > 0 {
		path = path + "?" + params.Encode()
	}

	body, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse task log response: %w", err)
	}

	// The log response is an array of log line objects
	logData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected task log response format")
	}

	var lines []string
	for _, item := range logData {
		logLine, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Each log entry has "n" (line number) and "t" (text)
		if text, ok := logLine["t"].(string); ok {
			lines = append(lines, text)
		}
	}

	return lines, nil
}

// warnPartialEnvOnce prints the partial environment warning only once per run
var warnPartialEnvOnce sync.Once

// ReadConfig reads configuration from environment variables or file
func ReadConfig() (string, string, string, error) {
	// Check environment variables first (for CI/CD scenarios)
	envURL := os.Getenv("PROX_URL")
	envUser := os.Getenv("PROX_USER")
	envPass := os.Getenv("PROX_PASSWORD")

	// If all environment variables are set, use them
	if envURL != "" && envUser != "" && envPass != "" {
		return envUser, envPass, envURL, nil
	}

	// Some but not all set is almost always a mistake; say why they're ignored
	if envURL != "" || envUser != "" || envPass != "" {
		warnPartialEnvOnce.Do(func() {
			fmt.Fprintln(os.Stderr, "Warning: PROX_URL, PROX_USER and PROX_PASSWORD must all be set to be used; only some are set, so the config file is used instead")
		})
	}

	// Fall back to config file
	return config.Read()
}

// CreateClient creates a new authenticated Proxmox client with caching
func CreateClient() (ProxmoxClientInterface, error) {
	user, pass, url, err := ReadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	tlsOpts, err := ResolveTLSOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to read TLS settings: %w", err)
	}

	// Create a cache key based on user, URL and TLS settings
	cacheKey := fmt.Sprintf("%s@%s|%v|%s", user, url, tlsOpts.Insecure, tlsOpts.CACert)

	// Try to get cached client first (read lock)
	clientCacheMutex.RLock()
	if cachedClient != nil && cachedClientKey == cacheKey {
		client := cachedClient
		clientCacheMutex.RUnlock()
		return client, nil
	}
	clientCacheMutex.RUnlock()

	// Need to create new client (write lock)
	clientCacheMutex.Lock()
	defer clientCacheMutex.Unlock()

	// Double-check after acquiring write lock
	if cachedClient != nil && cachedClientKey == cacheKey {
		return cachedClient, nil
	}

	// Create new client and cache it
	client, err := NewClientWithTLS(url, user, pass, tlsOpts)
	if err != nil {
		return nil, err
	}
	cachedClient = client
	cachedClientKey = cacheKey

	return client, nil
}

// ClearClientCache clears the cached client (useful for testing or config changes)
func ClearClientCache() {
	clientCacheMutex.Lock()
	defer clientCacheMutex.Unlock()
	cachedClient = nil
	cachedClientKey = ""
}

// ClearClusterResourcesCache clears the cluster resources cache for this client
func (c *ProxmoxClient) ClearClusterResourcesCache() {
	c.cachedResourcesMutex.Lock()
	defer c.cachedResourcesMutex.Unlock()
	c.cachedResources = nil
	c.cachedResourcesTime = time.Time{}
}

// GetVMNode finds which node a VM is running on
func (c *ProxmoxClient) GetVMNode(ctx context.Context, vmid int) (string, error) {
	resources, err := c.GetClusterResources(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get cluster resources: %w", err)
	}

	for _, resource := range resources {
		if resource.Type == "qemu" && resource.VMID != nil && *resource.VMID == vmid {
			return resource.Node, nil
		}
	}

	return "", fmt.Errorf("VM %d not found in cluster", vmid)
}

// getCachedIP retrieves a cached IP address if it exists and hasn't expired
func (c *ProxmoxClient) getCachedIP(node string, vmid int) (string, bool) {
	cacheKey := fmt.Sprintf("%s:%d", node, vmid)

	c.ipCacheMutex.RLock()
	defer c.ipCacheMutex.RUnlock()

	if entry, exists := c.ipCache[cacheKey]; exists {
		// Check if cache entry is still valid
		if time.Since(entry.timestamp).Seconds() < IPCacheTTL {
			return entry.ip, true
		}
	}

	return "", false
}

// setCachedIP stores an IP address in the cache with the current timestamp
func (c *ProxmoxClient) setCachedIP(node string, vmid int, ip string) {
	cacheKey := fmt.Sprintf("%s:%d", node, vmid)

	c.ipCacheMutex.Lock()
	defer c.ipCacheMutex.Unlock()

	c.ipCache[cacheKey] = ipCacheEntry{
		ip:        ip,
		timestamp: time.Now(),
	}
}

// GetVMIP gets the IP address of a VM using multiple methods
func (c *ProxmoxClient) GetVMIP(ctx context.Context, node string, vmid int) (string, error) {
	// Check cache first
	if cachedIP, found := c.getCachedIP(node, vmid); found {
		return cachedIP, nil
	}

	// Method 1: Try QEMU guest agent first (most reliable when available)
	if ip := c.getVMIPFromGuestAgent(ctx, node, vmid); ip != "N/A" {
		c.setCachedIP(node, vmid, ip)
		return ip, nil
	}

	// Method 2: Use the static IP configured with cloud-init, if any
	if ip := c.getVMIPFromConfig(ctx, node, vmid); ip != "N/A" {
		c.setCachedIP(node, vmid, ip)
		return ip, nil
	}

	// Cache "N/A" result to avoid repeated lookups for VMs without IPs
	c.setCachedIP(node, vmid, "N/A")
	return "N/A", nil
}

// extractIPFromInterfaces extracts IP address from guest agent network interfaces
// Prioritizes primary network interfaces (eth0, ens3, ens18, enp0s3) and filters loopback
func extractIPFromInterfaces(interfaces []interface{}) string {
	var primaryIP, anyIP string

	for _, iface := range interfaces {
		ifaceMap, ok := iface.(map[string]interface{})
		if !ok {
			continue
		}

		ifaceName, _ := ifaceMap["name"].(string)
		ipAddresses, ok := ifaceMap["ip-addresses"].([]interface{})
		if !ok {
			continue
		}

		for _, ipData := range ipAddresses {
			ipMap, ok := ipData.(map[string]interface{})
			if !ok {
				continue
			}

			ip, ok := ipMap["ip-address"].(string)
			if !ok {
				continue
			}

			ipType, ok := ipMap["ip-address-type"].(string)
			if !ok || ipType != "ipv4" {
				continue
			}

			// Skip loopback addresses
			if ip == "127.0.0.1" || ip == "::1" {
				continue
			}

			// Prioritize primary network interfaces
			if ifaceName == "eth0" || ifaceName == "ens3" || ifaceName == "ens18" || ifaceName == "enp0s3" {
				primaryIP = ip
				break
			}

			// Keep any valid IP as fallback
			if anyIP == "" {
				anyIP = ip
			}
		}

		if primaryIP != "" {
			break
		}
	}

	if primaryIP != "" {
		return primaryIP
	}
	if anyIP != "" {
		return anyIP
	}

	return "N/A"
}

// getVMIPFromGuestAgent tries to get IP from QEMU guest agent
func (c *ProxmoxClient) getVMIPFromGuestAgent(ctx context.Context, node string, vmid int) string {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", url.PathEscape(node), vmid)
	body, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return "N/A"
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "N/A"
	}

	result, ok := resp.Data.(map[string]interface{})
	if !ok {
		return "N/A"
	}

	interfaces, ok := result["result"].([]interface{})
	if !ok {
		return "N/A"
	}

	return extractIPFromInterfaces(interfaces)
}

// GetContainerIP gets the IP address of a container using multiple methods
func (c *ProxmoxClient) GetContainerIP(ctx context.Context, node string, vmid int) (string, error) {
	// Check cache first
	if cachedIP, found := c.getCachedIP(node, vmid); found {
		return cachedIP, nil
	}

	// Method 1: Try LXC guest agent first
	if ip := c.getContainerIPFromGuestAgent(ctx, node, vmid); ip != "N/A" {
		c.setCachedIP(node, vmid, ip)
		return ip, nil
	}

	// Method 2: Try alternative methods
	if ip, err := c.GetContainerIPAlternative(ctx, node, vmid); err == nil && ip != "N/A" {
		c.setCachedIP(node, vmid, ip)
		return ip, nil
	}

	// Cache "N/A" result to avoid repeated lookups
	c.setCachedIP(node, vmid, "N/A")
	return "N/A", nil
}

// getContainerIPFromGuestAgent tries to get IP from LXC guest agent
func (c *ProxmoxClient) getContainerIPFromGuestAgent(ctx context.Context, node string, vmid int) string {
	path := fmt.Sprintf("/nodes/%s/lxc/%d/agent/network-get-interfaces", url.PathEscape(node), vmid)
	body, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return "N/A"
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "N/A"
	}

	result, ok := resp.Data.(map[string]interface{})
	if !ok {
		return "N/A"
	}

	interfaces, ok := result["result"].([]interface{})
	if !ok {
		return "N/A"
	}

	return extractIPFromInterfaces(interfaces)
}

// GetContainerIPAlternative tries to get container IP from various sources
func (c *ProxmoxClient) GetContainerIPAlternative(ctx context.Context, node string, vmid int) (string, error) {
	// IMPORTANT: Do not call GetContainerIP here to avoid infinite recursion.

	// Check cache first
	if cachedIP, found := c.getCachedIP(node, vmid); found {
		return cachedIP, nil
	}

	// Try to get IP from container status
	status, err := c.GetContainerStatus(ctx, node, vmid)
	if err != nil {
		return "N/A", nil
	}

	// Check if there's IP information in the status
	if ips, exists := status["ips"]; exists {
		if ipStr, ok := ips.(string); ok && ipStr != "" {
			c.setCachedIP(node, vmid, ipStr)
			return ipStr, nil
		}
	}

	// Try to get IP from network interfaces endpoint
	path := fmt.Sprintf("/nodes/%s/lxc/%d/interfaces", url.PathEscape(node), vmid)
	body, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		c.setCachedIP(node, vmid, "N/A")
		return "N/A", nil
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		c.setCachedIP(node, vmid, "N/A")
		return "N/A", nil
	}

	if interfaces, ok := resp.Data.([]interface{}); ok {
		for _, iface := range interfaces {
			if ifaceMap, ok := iface.(map[string]interface{}); ok {
				if inet, exists := ifaceMap["inet"]; exists {
					if inetStr, ok := inet.(string); ok && inetStr != "" {
						// Parse IP from CIDR notation if needed
						if strings.Contains(inetStr, "/") {
							parts := strings.Split(inetStr, "/")
							if len(parts) > 0 && parts[0] != "127.0.0.1" {
								c.setCachedIP(node, vmid, parts[0])
								return parts[0], nil
							}
						} else if inetStr != "127.0.0.1" {
							c.setCachedIP(node, vmid, inetStr)
							return inetStr, nil
						}
					}
				}
			}
		}
	}

	// Cache "N/A" result to avoid repeated lookups
	c.setCachedIP(node, vmid, "N/A")
	return "N/A", nil
}

// GetNodeIP attempts to get a primary IP address for a Proxmox node.
// This uses the node network API and returns the first non-loopback IPv4 it finds.
func (c *ProxmoxClient) GetNodeIP(ctx context.Context, node string) (string, error) {
	path := fmt.Sprintf("/nodes/%s/network", url.PathEscape(node))
	body, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return "N/A", err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "N/A", err
	}

	if ifaces, ok := resp.Data.([]interface{}); ok {
		for _, iface := range ifaces {
			ifm, ok := iface.(map[string]interface{})
			if !ok {
				continue
			}

			// try common fields: address, cidr, inet
			if addr, ok := ifm["address"].(string); ok && addr != "" && addr != "127.0.0.1" {
				// some responses include CIDR; strip if present
				if strings.Contains(addr, "/") {
					return strings.Split(addr, "/")[0], nil
				}
				return addr, nil
			}

			if cidr, ok := ifm["cidr"].(string); ok && cidr != "" {
				if strings.Contains(cidr, "/") {
					parts := strings.Split(cidr, "/")
					if parts[0] != "127.0.0.1" {
						return parts[0], nil
					}
				}
			}

			if inet, ok := ifm["inet"].(string); ok && inet != "" && inet != "127.0.0.1" {
				if strings.Contains(inet, "/") {
					return strings.Split(inet, "/")[0], nil
				}
				return inet, nil
			}
		}
	}

	return "N/A", nil
}

// UpdateVM updates a virtual machine configuration
func (c *ProxmoxClient) UpdateVM(ctx context.Context, node string, vmid int, config map[string]interface{}) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid)

	body, err := c.makeRequest(ctx, "PUT", path, config)
	if err != nil {
		return "", err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse update VM response: %w", err)
	}

	// For VM config updates, Proxmox might return a task ID or just success
	if taskID, ok := resp.Data.(string); ok {
		return taskID, nil
	}

	// If no task ID is returned, it was likely a synchronous operation
	return "", nil
}

// ResizeDisk resizes a VM disk
func (c *ProxmoxClient) ResizeDisk(ctx context.Context, node string, vmid int, disk string, size string) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/resize", url.PathEscape(node), vmid)
	reqBody := map[string]interface{}{
		"disk": disk,
		"size": size,
	}

	return requestData[string](ctx, c, "PUT", path, reqBody, "resize disk")
}

// GetVMConfig gets the configuration of a virtual machine
func (c *ProxmoxClient) GetVMConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid)

	return requestData[map[string]interface{}](ctx, c, "GET", path, nil, "VM config")
}

// GetStorageNames returns the names of the active storages on node that can
// hold the given content type (e.g. "rootdir" for container root disks)
func (c *ProxmoxClient) GetStorageNames(ctx context.Context, node string, content string) ([]string, error) {
	path := fmt.Sprintf("/nodes/%s/storage?enabled=1&content=%s", url.PathEscape(node), url.QueryEscape(content))
	list, err := requestData[[]interface{}](ctx, c, "GET", path, nil, "storage")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, item := range list {
		storage, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if active, ok := storage["active"].(float64); ok && active == 0 {
			continue
		}
		if name, ok := storage["storage"].(string); ok && name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

// GetContainerTemplates gets available container templates from a node
func (c *ProxmoxClient) GetContainerTemplates(ctx context.Context, node string) ([]ContainerTemplate, error) {
	path := fmt.Sprintf("/nodes/%s/storage", url.PathEscape(node))

	body, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse storage response: %w", err)
	}

	storageList, ok := resp.Data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected storage response format")
	}

	var allTemplates []ContainerTemplate

	// Check each storage for container templates
	for _, storage := range storageList {
		storageMap, ok := storage.(map[string]interface{})
		if !ok {
			continue
		}

		storageID, ok := storageMap["storage"].(string)
		if !ok {
			continue
		}

		// Check if this storage contains container templates
		content, ok := storageMap["content"].(string)
		if !ok || !strings.Contains(content, "vztmpl") {
			continue
		}

		// Get templates from this storage
		templates, err := c.getStorageTemplates(ctx, node, storageID)
		if err != nil {
			// Continue with other storages if one fails
			continue
		}

		allTemplates = append(allTemplates, templates...)
	}

	return allTemplates, nil
}

// getStorageTemplates gets templates from a specific storage
func (c *ProxmoxClient) getStorageTemplates(ctx context.Context, node, storage string) ([]ContainerTemplate, error) {
	path := fmt.Sprintf("/nodes/%s/storage/%s/content", url.PathEscape(node), url.PathEscape(storage))

	// Add query parameter to filter for container templates
	query := "?content=vztmpl"

	body, err := c.makeRequest(ctx, "GET", path+query, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse storage content response: %w", err)
	}

	contentList, ok := resp.Data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected storage content response format")
	}

	var templates []ContainerTemplate
	for _, content := range contentList {
		contentMap, ok := content.(map[string]interface{})
		if !ok {
			continue
		}

		volID, ok := contentMap["volid"].(string)
		if !ok {
			continue
		}

		size, _ := contentMap["size"].(float64)
		used, _ := contentMap["used"].(float64)

		template := ContainerTemplate{
			VolID: volID,
			Size:  uint64(size),
			Used:  uint64(used),
		}

		templates = append(templates, template)
	}

	return templates, nil
}

// GetVMStatus gets the current status of a virtual machine
func (c *ProxmoxClient) GetVMStatus(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/current", url.PathEscape(node), vmid)
	return requestData[map[string]interface{}](ctx, c, "GET", path, nil, "VM status")
}

// GetVMDiskInfo gets more detailed disk information from VM configuration
func (c *ProxmoxClient) GetVMDiskInfo(ctx context.Context, node string, vmid int) (uint64, uint64, error) {
	config, err := c.GetVMConfig(ctx, node, vmid)
	if err != nil {
		return 0, 0, err
	}

	var totalSize uint64 = 0
	var usedSize uint64 = 0

	// Cache for storage content to avoid repeated API calls for the same storage
	storageCache := make(map[string][]interface{})

	// Check all possible disk types
	diskKeys := []string{"ide0", "ide1", "ide2", "ide3", "sata0", "sata1", "sata2", "sata3", "scsi0", "scsi1", "scsi2", "scsi3", "virtio0", "virtio1", "virtio2", "virtio3", "efidisk0", "tpmstate0"}

	for _, diskKey := range diskKeys {
		if diskConfig, exists := config[diskKey]; exists {
			if diskStr, ok := diskConfig.(string); ok {
				// Skip if it's a cdrom/iso
				if strings.Contains(diskStr, "media=cdrom") || strings.Contains(diskStr, ".iso") {
					continue
				}

				// Parse disk size from config string
				// Format can be: "local-lvm:vm-100-disk-0,size=32G" or "storage:vm-100-disk-0.raw"
				parts := strings.Split(diskStr, ",")
				diskSize := uint64(0)

				// Look for explicit size parameter
				for _, part := range parts {
					if strings.HasPrefix(part, "size=") {
						sizeStr := strings.TrimPrefix(part, "size=")
						diskSize = parseDiskSize(sizeStr)
						break
					}
				}

				// If no explicit size found, try to get disk size from storage API
				if diskSize == 0 {
					// Extract storage and volume ID from the disk string
					if colonIndex := strings.Index(diskStr, ":"); colonIndex != -1 {
						storage := diskStr[:colonIndex]
						volumeID := diskStr[colonIndex+1:]

						// Remove any additional parameters
						if commaIndex := strings.Index(volumeID, ","); commaIndex != -1 {
							volumeID = volumeID[:commaIndex]
						}

						// Try to get disk size from storage API (with caching)
						if storageSize, err := c.getStorageVolumeSizeWithCache(ctx, node, storage, volumeID, storageCache); err == nil {
							diskSize = storageSize
						}
					}
				}

				totalSize += diskSize
			}
		}
	}

	// Disk usage: Proxmox only reports it for some setups, so fall back to the
	// QEMU guest agent. If neither is available, usage is unknown and 0 is
	// returned; callers must show that as unknown rather than as empty.
	status, err := c.GetVMStatus(ctx, node, vmid)
	if err == nil {
		if diskUsed, ok := status["disk"].(float64); ok {
			usedSize = uint64(diskUsed)
		}

		// If we still don't have total size, try to get it from status
		if totalSize == 0 {
			if diskMax, ok := status["maxdisk"].(float64); ok {
				totalSize = uint64(diskMax)
			}
		}

		if usedSize == 0 && status["status"] == "running" {
			if agentUsed, err := c.getVMDiskUsageFromGuestAgent(ctx, node, vmid); err == nil {
				usedSize = agentUsed
			}
		}
	}

	return totalSize, usedSize, nil
}

// getVMDiskUsageFromGuestAgent returns the space used on the VM's disk-backed
// filesystems as reported by the QEMU guest agent.
func (c *ProxmoxClient) getVMDiskUsageFromGuestAgent(ctx context.Context, node string, vmid int) (uint64, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/agent/get-fsinfo", url.PathEscape(node), vmid)
	body, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return 0, err
	}

	var resp struct {
		Data struct {
			Result []guestFilesystem `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("failed to parse guest agent fsinfo: %w", err)
	}

	used, ok := sumGuestFilesystemUsage(resp.Data.Result)
	if !ok {
		return 0, fmt.Errorf("guest agent reported no disk usage")
	}
	return used, nil
}

// guestFilesystem is one entry from the guest agent's get-fsinfo command
type guestFilesystem struct {
	Name      string            `json:"name"`
	UsedBytes *uint64           `json:"used-bytes"`
	Disk      []json.RawMessage `json:"disk"`
}

// sumGuestFilesystemUsage adds up used bytes for filesystems backed by a disk,
// counting each filesystem once (bind mounts report the same filesystem again).
func sumGuestFilesystemUsage(filesystems []guestFilesystem) (uint64, bool) {
	var total uint64
	found := false
	seen := make(map[string]bool)
	for _, fs := range filesystems {
		if fs.UsedBytes == nil || len(fs.Disk) == 0 || seen[fs.Name] {
			continue
		}
		seen[fs.Name] = true
		total += *fs.UsedBytes
		found = true
	}
	return total, found
}

// getStorageVolumeSizeWithCache is like getStorageVolumeSize but uses a cache to avoid
// repeated API calls for the same storage. This significantly improves performance when
// a VM has multiple disks on the same storage.
func (c *ProxmoxClient) getStorageVolumeSizeWithCache(ctx context.Context, node, storage, volumeID string, cache map[string][]interface{}) (uint64, error) {
	cacheKey := fmt.Sprintf("%s:%s", node, storage)

	// Check if we have cached content for this storage
	contentList, cached := cache[cacheKey]

	if !cached {
		// Fetch storage content from API
		path := fmt.Sprintf("/nodes/%s/storage/%s/content", url.PathEscape(node), url.PathEscape(storage))

		body, err := c.makeRequest(ctx, "GET", path, nil)
		if err != nil {
			return 0, err
		}

		var resp APIResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return 0, err
		}

		if list, ok := resp.Data.([]interface{}); ok {
			contentList = list
			// Cache the content for future lookups
			cache[cacheKey] = contentList
		} else {
			return 0, fmt.Errorf("unexpected storage content format")
		}
	}

	// Search for the volume in the cached content
	for _, content := range contentList {
		if contentMap, ok := content.(map[string]interface{}); ok {
			if volID, exists := contentMap["volid"]; exists {
				if volIDStr, ok := volID.(string); ok {
					// Match the volume ID - try exact match first, then partial match
					if volIDStr == fmt.Sprintf("%s:%s", storage, volumeID) || strings.Contains(volIDStr, volumeID) {
						if size, exists := contentMap["size"]; exists {
							if sizeFloat, ok := size.(float64); ok {
								return uint64(sizeFloat), nil
							}
						}
					}
				}
			}
		}
	}

	return 0, fmt.Errorf("volume not found in storage")
}

// parseDiskSize parses disk size strings like "32G", "1024M", "2T"
func parseDiskSize(sizeStr string) uint64 {
	if len(sizeStr) == 0 {
		return 0
	}

	// Get the last character (unit)
	unit := sizeStr[len(sizeStr)-1:]
	numStr := sizeStr[:len(sizeStr)-1]

	// Parse the number
	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0
	}

	// Convert based on unit
	switch strings.ToUpper(unit) {
	case "K":
		return uint64(num * 1024)
	case "M":
		return uint64(num * 1024 * 1024)
	case "G":
		return uint64(num * 1024 * 1024 * 1024)
	case "T":
		return uint64(num * 1024 * 1024 * 1024 * 1024)
	default:
		// If no unit, assume bytes
		return uint64(num)
	}
}

// getVMIPFromConfig returns a static IPv4 address configured with cloud-init
// (ipconfig0, ipconfig1, ...), e.g. "ip=192.168.1.100/24,gw=192.168.1.1".
func (c *ProxmoxClient) getVMIPFromConfig(ctx context.Context, node string, vmid int) string {
	config, err := c.GetVMConfig(ctx, node, vmid)
	if err != nil {
		return "N/A"
	}
	return ipFromCloudInitConfig(config)
}

// ipFromCloudInitConfig picks the static IPv4 address from the lowest numbered
// ipconfigN entry that has one
func ipFromCloudInitConfig(config map[string]interface{}) string {
	for i := 0; i < 32; i++ {
		value, ok := config[fmt.Sprintf("ipconfig%d", i)].(string)
		if !ok {
			continue
		}
		for _, part := range strings.Split(value, ",") {
			ipWithMask, found := strings.CutPrefix(strings.TrimSpace(part), "ip=")
			if !found {
				continue
			}
			ip, _, _ := strings.Cut(ipWithMask, "/")
			if ip != "" && ip != "dhcp" && ip != "127.0.0.1" {
				return ip
			}
		}
	}
	return "N/A"
}

// MigrateVM migrates a virtual machine to another node
func (c *ProxmoxClient) MigrateVM(ctx context.Context, node string, vmid int, targetNode string, options map[string]interface{}) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/migrate", url.PathEscape(node), vmid)

	// Prepare migration parameters
	reqBody := map[string]interface{}{
		"target": targetNode,
	}

	// Add optional parameters if provided
	for key, value := range options {
		reqBody[key] = value
	}

	return requestData[string](ctx, c, "POST", path, reqBody, "migrate VM")
}
