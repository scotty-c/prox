package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

	"github.com/scotty-c/prox/pkg/apierr"
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

	// Operations for either guest type (GuestVM or GuestContainer)
	RebootGuest(ctx context.Context, node, guestType string, vmid int) (string, error)

	// More container operations
	CloneContainer(ctx context.Context, node string, vmid int, newid int, hostname string, full bool) (string, error)
	MigrateContainer(ctx context.Context, node string, vmid int, targetNode string, restart bool) (string, error)
	UpdateContainer(ctx context.Context, node string, vmid int, config map[string]interface{}) error
	ResizeContainerDisk(ctx context.Context, node string, vmid int, disk, size string) (string, error)
	GetAvailableTemplates(ctx context.Context, node string) ([]Appliance, error)
	DownloadTemplate(ctx context.Context, node, storage, template string) (string, error)

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
	GetNodeAddress(ctx context.Context, node string) (string, error)

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
			if errors.Is(err, apierr.ErrAuth) || errors.Is(err, apierr.ErrPermission) {
				return fmt.Errorf("API token rejected: %w", err)
			}
			return err
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
		return c.transportError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read auth response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &APIError{Status: resp.StatusCode, Message: apiErrorMessage(strings.TrimSpace(strings.TrimPrefix(resp.Status, strconv.Itoa(resp.StatusCode))), body), login: true}
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
			return nil, err
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
			return nil, &APIError{Status: status, Message: apiErrorMessage(reason, respBody)}
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
		return 0, "", nil, c.transportError(err)
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

// APIError is an error response from the Proxmox API. It matches
// apierr.ErrAuth for 401 and apierr.ErrPermission for 403.
type APIError struct {
	Status  int
	Message string
	login   bool // the failed request was the login itself
}

func (e *APIError) Error() string {
	if e.login {
		return fmt.Sprintf("authentication failed with status %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("API request failed with status %d: %s", e.Status, e.Message)
}

// Is lets errors.Is classify the response by its status code
func (e *APIError) Is(target error) bool {
	switch target {
	case apierr.ErrAuth:
		return e.Status == http.StatusUnauthorized
	case apierr.ErrPermission:
		return e.Status == http.StatusForbidden
	}
	return false
}

// transportError explains a failure to get any response from the server. A
// cancelled context (Ctrl-C) is returned as it is; certificate problems get
// instructions; anything else means the server could not be reached.
func (c *ProxmoxClient) transportError(err error) error {
	if errors.Is(err, context.Canceled) {
		return err
	}
	if wrapped := wrapTLSError(err, c.BaseURL); wrapped != err {
		return wrapped
	}
	return fmt.Errorf("%w at %s: %w", apierr.ErrUnreachable, c.BaseURL, err)
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
	v, err := requestData[Version](ctx, c, "GET", "/version", nil, "version")
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// GetNodes gets the list of cluster nodes
func (c *ProxmoxClient) GetNodes(ctx context.Context) ([]Node, error) {
	return requestData[[]Node](ctx, c, "GET", "/nodes", nil, "nodes")
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

// requestData makes an API request and decodes the response's "data" field
// into T. A task ID is a string that must not be empty, and a config or status
// map must not be missing; both are reported as an unexpected response.
func requestData[T any](ctx context.Context, c *ProxmoxClient, method, path string, body interface{}, action string) (T, error) {
	var resp struct {
		Data T `json:"data"`
	}
	raw, err := c.makeRequest(ctx, method, path, body)
	if err != nil {
		return resp.Data, err
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return resp.Data, fmt.Errorf("failed to parse %s response: %w", action, err)
	}
	switch data := any(resp.Data).(type) {
	case string:
		if data == "" {
			return resp.Data, fmt.Errorf("unexpected %s response format: no data", action)
		}
	case map[string]interface{}:
		if data == nil {
			return resp.Data, fmt.Errorf("unexpected %s response format: no data", action)
		}
	}
	return resp.Data, nil
}

// requestOptionalTask makes an API request that either starts a task or just
// succeeds (Proxmox returns a task ID for some changes and null for others).
// It returns the task ID, or "" if there was none.
func requestOptionalTask(ctx context.Context, c *ProxmoxClient, method, path string, body interface{}, action string) (string, error) {
	raw, err := c.makeRequest(ctx, method, path, body)
	if err != nil {
		return "", err
	}
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("failed to parse %s response: %w", action, err)
	}
	var taskID string
	_ = json.Unmarshal(resp.Data, &taskID) // null or not a string: no task
	return taskID, nil
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
	id, err := requestData[flexInt](ctx, c, "GET", "/cluster/nextid", nil, "next ID")
	return int(id), err
}

// flexInt is an integer that Proxmox may send as a number or as a string
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	n, err := strconv.Atoi(strings.Trim(string(b), `"`))
	if err != nil {
		return fmt.Errorf("invalid number %s", b)
	}
	*f = flexInt(n)
	return nil
}

// GetTaskStatus gets the status of a Proxmox task
func (c *ProxmoxClient) GetTaskStatus(ctx context.Context, node, upid string) (*Task, error) {
	path := fmt.Sprintf("/nodes/%s/tasks/%s/status", url.PathEscape(node), url.PathEscape(upid))
	task, err := requestData[Task](ctx, c, "GET", path, nil, "task status")
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// taskLogPageSize is how many log lines are requested at a time
const taskLogPageSize = 500

// GetTaskLog retrieves the log for a Proxmox task, starting at line start.
// With limit > 0 it returns at most limit lines. With limit <= 0 it returns
// every line from start to the end of the log: Proxmox returns only 50 lines
// when no limit is sent, so the log is read in pages.
func (c *ProxmoxClient) GetTaskLog(ctx context.Context, node, upid string, start int, limit int) ([]string, error) {
	if limit > 0 {
		return c.getTaskLogPage(ctx, node, upid, start, limit)
	}

	var all []string
	for {
		page, err := c.getTaskLogPage(ctx, node, upid, start+len(all), taskLogPageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < taskLogPageSize {
			return all, nil
		}
	}
}

// getTaskLogPage reads one page of a task's log
func (c *ProxmoxClient) getTaskLogPage(ctx context.Context, node, upid string, start int, limit int) ([]string, error) {
	params := url.Values{}
	params.Add("limit", strconv.Itoa(limit))
	if start > 0 {
		params.Add("start", strconv.Itoa(start))
	}
	path := fmt.Sprintf("/nodes/%s/tasks/%s/log?%s", url.PathEscape(node), url.PathEscape(upid), params.Encode())

	entries, err := requestData[[]struct {
		Text string `json:"t"`
	}](ctx, c, "GET", path, nil, "task log")
	if err != nil {
		return nil, err
	}
	lines := make([]string, len(entries))
	for i, e := range entries {
		lines[i] = e.Text
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

// GetVMIP gets the IP address of a VM, or "" if it has none that can be found
func (c *ProxmoxClient) GetVMIP(ctx context.Context, node string, vmid int) (string, error) {
	if cachedIP, found := c.getCachedIP(node, vmid); found {
		return cachedIP, nil
	}

	// The QEMU guest agent is the most reliable source, when it is running
	ip := c.guestAgentIP(ctx, node, GuestVM, vmid)
	if ip == "" {
		// Otherwise use the static address configured with cloud-init, if any
		ip = c.getVMIPFromConfig(ctx, node, vmid)
	}

	// Remember "no address" too, so VMs without one are not looked up again
	c.setCachedIP(node, vmid, ip)
	return ip, nil
}

// agentInterface is a network interface as reported by a guest agent
type agentInterface struct {
	Name        string `json:"name"`
	IPAddresses []struct {
		Address string `json:"ip-address"`
		Type    string `json:"ip-address-type"`
	} `json:"ip-addresses"`
}

// extractIPFromInterfaces picks an IPv4 address from guest agent interfaces,
// preferring the usual primary interface names (eth0, ens3, ens18, enp0s3) and
// skipping loopback. It returns "" if there is none.
func extractIPFromInterfaces(interfaces []agentInterface) string {
	primaryNames := map[string]bool{"eth0": true, "ens3": true, "ens18": true, "enp0s3": true}
	var anyIP string

	for _, iface := range interfaces {
		for _, addr := range iface.IPAddresses {
			if addr.Type != "ipv4" || addr.Address == "127.0.0.1" || addr.Address == "::1" || addr.Address == "" {
				continue
			}
			if primaryNames[iface.Name] {
				return addr.Address
			}
			if anyIP == "" {
				anyIP = addr.Address
			}
		}
	}
	return anyIP
}

// guestAgentIP asks a guest agent for the guest's IPv4 address, or "" if it cannot
func (c *ProxmoxClient) guestAgentIP(ctx context.Context, node, guestType string, vmid int) string {
	path := fmt.Sprintf("/nodes/%s/%s/%d/agent/network-get-interfaces", url.PathEscape(node), guestType, vmid)
	data, err := requestData[struct {
		Result []agentInterface `json:"result"`
	}](ctx, c, "GET", path, nil, "guest agent interfaces")
	if err != nil {
		return ""
	}
	return extractIPFromInterfaces(data.Result)
}

// GetContainerIP gets the IP address of a container, or "" if it has none that can be found
func (c *ProxmoxClient) GetContainerIP(ctx context.Context, node string, vmid int) (string, error) {
	if cachedIP, found := c.getCachedIP(node, vmid); found {
		return cachedIP, nil
	}

	ip := c.guestAgentIP(ctx, node, GuestContainer, vmid)
	if ip == "" {
		ip, _ = c.GetContainerIPAlternative(ctx, node, vmid)
	}

	c.setCachedIP(node, vmid, ip)
	return ip, nil
}

// GetContainerIPAlternative tries to get container IP from various sources
func (c *ProxmoxClient) GetContainerIPAlternative(ctx context.Context, node string, vmid int) (string, error) {
	// IMPORTANT: Do not call GetContainerIP here to avoid infinite recursion.
	if cachedIP, found := c.getCachedIP(node, vmid); found {
		return cachedIP, nil
	}

	status, err := c.GetContainerStatus(ctx, node, vmid)
	if err != nil {
		return "", nil
	}
	if ips, ok := status["ips"].(string); ok && ips != "" {
		c.setCachedIP(node, vmid, ips)
		return ips, nil
	}

	interfaces, err := requestData[[]struct {
		Inet string `json:"inet"`
	}](ctx, c, "GET", fmt.Sprintf("/nodes/%s/lxc/%d/interfaces", url.PathEscape(node), vmid), nil, "container interfaces")
	ip := ""
	if err == nil {
		for _, iface := range interfaces {
			if addr := addressOf(iface.Inet); addr != "" && addr != "127.0.0.1" {
				ip = addr
				break
			}
		}
	}

	c.setCachedIP(node, vmid, ip)
	return ip, nil
}

// addressOf returns the address part of "10.0.0.5/24" (or of a bare address)
func addressOf(cidr string) string {
	addr, _, _ := strings.Cut(strings.TrimSpace(cidr), "/")
	return addr
}

// GetNodeIP attempts to get a primary IP address for a Proxmox node, or ""
// if it has none. It uses the node network API and returns the first
// non-loopback address it finds.
func (c *ProxmoxClient) GetNodeIP(ctx context.Context, node string) (string, error) {
	interfaces, err := requestData[[]struct {
		Address string `json:"address"`
		CIDR    string `json:"cidr"`
		Inet    string `json:"inet"`
	}](ctx, c, "GET", fmt.Sprintf("/nodes/%s/network", url.PathEscape(node)), nil, "node network")
	if err != nil {
		return "", err
	}
	for _, iface := range interfaces {
		for _, candidate := range []string{iface.Address, iface.CIDR, iface.Inet} {
			if addr := addressOf(candidate); addr != "" && addr != "127.0.0.1" {
				return addr, nil
			}
		}
	}
	return "", nil
}

// UpdateVM updates a virtual machine configuration
func (c *ProxmoxClient) UpdateVM(ctx context.Context, node string, vmid int, config map[string]interface{}) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid)
	// Proxmox returns a task ID for some changes and nothing for others
	return requestOptionalTask(ctx, c, "PUT", path, config, "update VM")
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
	storages, err := requestData[[]struct {
		Storage string      `json:"storage"`
		Active  interface{} `json:"active"`
	}](ctx, c, "GET", path, nil, "storage")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, st := range storages {
		if active, ok := ConfigFlag(st.Active); ok && !active {
			continue // listed but not usable right now
		}
		if st.Storage != "" {
			names = append(names, st.Storage)
		}
	}
	return names, nil
}

// GetContainerTemplates gets available container templates from a node
func (c *ProxmoxClient) GetContainerTemplates(ctx context.Context, node string) ([]ContainerTemplate, error) {
	path := fmt.Sprintf("/nodes/%s/storage", url.PathEscape(node))
	storages, err := requestData[[]struct {
		Storage string `json:"storage"`
		Content string `json:"content"`
	}](ctx, c, "GET", path, nil, "storage")
	if err != nil {
		return nil, err
	}

	var all []ContainerTemplate
	for _, st := range storages {
		if st.Storage == "" || !strings.Contains(st.Content, "vztmpl") {
			continue
		}
		templates, err := c.getStorageTemplates(ctx, node, st.Storage)
		if err != nil {
			continue // keep going with the other storages
		}
		all = append(all, templates...)
	}
	return all, nil
}

// getStorageTemplates gets templates from a specific storage
func (c *ProxmoxClient) getStorageTemplates(ctx context.Context, node, storage string) ([]ContainerTemplate, error) {
	path := fmt.Sprintf("/nodes/%s/storage/%s/content?content=vztmpl", url.PathEscape(node), url.PathEscape(storage))
	items, err := requestData[[]struct {
		VolID string  `json:"volid"`
		Size  float64 `json:"size"`
		Used  float64 `json:"used"`
	}](ctx, c, "GET", path, nil, "storage content")
	if err != nil {
		return nil, err
	}
	var templates []ContainerTemplate
	for _, it := range items {
		if it.VolID != "" {
			templates = append(templates, ContainerTemplate{VolID: it.VolID, Size: uint64(it.Size), Used: uint64(it.Used)})
		}
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
	storageCache := make(map[string][]storageVolume)

	// Check all possible disk types
	diskKeys := append(DiskKeys(config), "efidisk0", "tpmstate0")

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

// storageVolume is one entry of a storage's content listing
type storageVolume struct {
	VolID string  `json:"volid"`
	Size  float64 `json:"size"`
}

// getStorageVolumeSizeWithCache returns the size of a volume. cache holds the
// content listings already fetched, keyed by node and storage, so a VM with
// several disks on one storage costs one request.
func (c *ProxmoxClient) getStorageVolumeSizeWithCache(ctx context.Context, node, storage, volumeID string, cache map[string][]storageVolume) (uint64, error) {
	cacheKey := fmt.Sprintf("%s:%s", node, storage)

	volumes, cached := cache[cacheKey]
	if !cached {
		path := fmt.Sprintf("/nodes/%s/storage/%s/content", url.PathEscape(node), url.PathEscape(storage))
		var err error
		if volumes, err = requestData[[]storageVolume](ctx, c, "GET", path, nil, "storage content"); err != nil {
			return 0, err
		}
		cache[cacheKey] = volumes
	}

	// Exact match only: a substring match would let vm-100-disk-1 pick up
	// vm-100-disk-10
	want := fmt.Sprintf("%s:%s", storage, volumeID)
	for _, v := range volumes {
		if v.VolID == want {
			return uint64(v.Size), nil
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
		return ""
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
	return ""
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
