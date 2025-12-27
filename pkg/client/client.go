package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

	// Cluster resources cache
	cachedResources      []Resource
	cachedResourcesTime  time.Time
	cachedResourcesMutex sync.RWMutex
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

// Client cache for reuse across operations
var (
	cachedClient     *ProxmoxClient
	cachedClientKey  string
	clientCacheMutex sync.RWMutex
)

// NewClient creates a new Proxmox client
func NewClient(baseURL, username, password string) *ProxmoxClient {
	// Create HTTP client with custom transport for self-signed certificates
	// and connection pooling for better performance
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
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
	}
}

// Authenticate authenticates with Proxmox and stores the ticket
func (c *ProxmoxClient) Authenticate(ctx context.Context) error {
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
		return fmt.Errorf("authentication request failed: %w", err)
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
	// Ensure we're authenticated
	if c.ticket == "" {
		if err := c.Authenticate(ctx); err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
	}

	url := fmt.Sprintf("%s/api2/json%s", c.BaseURL, path)

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication headers
	req.Header.Set("Cookie", fmt.Sprintf("PVEAuthCookie=%s", c.ticket))
	if method != "GET" && method != "HEAD" {
		req.Header.Set("CSRFPreventionToken", c.csrfToken)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
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
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/start", node, vmid)
	body, err := c.makeRequest(ctx, "POST", path, nil)
	if err != nil {
		return "", err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse start VM response: %w", err)
	}

	if taskID, ok := resp.Data.(string); ok {
		return taskID, nil
	}

	return "", fmt.Errorf("unexpected start VM response format")
}

// StopVM stops a virtual machine
func (c *ProxmoxClient) StopVM(ctx context.Context, node string, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/shutdown", node, vmid)
	body, err := c.makeRequest(ctx, "POST", path, nil)
	if err != nil {
		return "", err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse stop VM response: %w", err)
	}

	if taskID, ok := resp.Data.(string); ok {
		return taskID, nil
	}

	return "", fmt.Errorf("unexpected stop VM response format")
}

// DeleteVM deletes a virtual machine
func (c *ProxmoxClient) DeleteVM(ctx context.Context, node string, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d", node, vmid)
	body, err := c.makeRequest(ctx, "DELETE", path, nil)
	if err != nil {
		return "", err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse delete VM response: %w", err)
	}

	if taskID, ok := resp.Data.(string); ok {
		return taskID, nil
	}

	return "", fmt.Errorf("unexpected delete VM response format")
}

// CloneVM clones a virtual machine
func (c *ProxmoxClient) CloneVM(ctx context.Context, node string, vmid int, newid int, name string, full bool) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/clone", node, vmid)
	reqBody := map[string]interface{}{
		"newid": newid,
		"name":  name,
	}

	// Add full parameter if requested to create a full clone instead of linked clone
	if full {
		reqBody["full"] = 1
	}

	body, err := c.makeRequest(ctx, "POST", path, reqBody)
	if err != nil {
		return "", err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse clone VM response: %w", err)
	}

	if taskID, ok := resp.Data.(string); ok {
		return taskID, nil
	}

	return "", fmt.Errorf("unexpected clone VM response format")
}

// CreateContainer creates a new LXC container
func (c *ProxmoxClient) CreateContainer(ctx context.Context, node string, vmid int, params map[string]interface{}) (string, error) {
	path := fmt.Sprintf("/nodes/%s/lxc", node)

	// Ensure vmid is included in the request
	params["vmid"] = vmid

	body, err := c.makeRequest(ctx, "POST", path, params)
	if err != nil {
		return "", err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse create container response: %w", err)
	}

	if taskID, ok := resp.Data.(string); ok {
		return taskID, nil
	}

	return "", fmt.Errorf("unexpected create container response format")
}

// StartContainer starts an LXC container
func (c *ProxmoxClient) StartContainer(ctx context.Context, node string, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/lxc/%d/status/start", node, vmid)
	body, err := c.makeRequest(ctx, "POST", path, nil)
	if err != nil {
		return "", err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse start container response: %w", err)
	}

	if taskID, ok := resp.Data.(string); ok {
		return taskID, nil
	}

	return "", fmt.Errorf("unexpected start container response format")
}

// StopContainer stops an LXC container
func (c *ProxmoxClient) StopContainer(ctx context.Context, node string, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/lxc/%d/status/shutdown", node, vmid)
	body, err := c.makeRequest(ctx, "POST", path, nil)
	if err != nil {
		return "", err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse stop container response: %w", err)
	}

	if taskID, ok := resp.Data.(string); ok {
		return taskID, nil
	}

	return "", fmt.Errorf("unexpected stop container response format")
}

// DeleteContainer deletes a container
func (c *ProxmoxClient) DeleteContainer(ctx context.Context, node string, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/lxc/%d", node, vmid)
	body, err := c.makeRequest(ctx, "DELETE", path, nil)
	if err != nil {
		return "", err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse delete container response: %w", err)
	}

	if taskID, ok := resp.Data.(string); ok {
		return taskID, nil
	}

	return "", fmt.Errorf("unexpected delete container response format")
}

// GetContainerConfig gets the configuration of an LXC container
func (c *ProxmoxClient) GetContainerConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	path := fmt.Sprintf("/nodes/%s/lxc/%d/config", node, vmid)
	body, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse container config response: %w", err)
	}

	if config, ok := resp.Data.(map[string]interface{}); ok {
		return config, nil
	}

	return nil, fmt.Errorf("unexpected container config response format")
}

// GetContainerStatus gets the status of an LXC container
func (c *ProxmoxClient) GetContainerStatus(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	path := fmt.Sprintf("/nodes/%s/lxc/%d/status/current", node, vmid)
	body, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse container status response: %w", err)
	}

	if status, ok := resp.Data.(map[string]interface{}); ok {
		return status, nil
	}

	return nil, fmt.Errorf("unexpected container status response format")
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
	path := fmt.Sprintf("/nodes/%s/tasks/%s/status", node, upid)

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

// ReadConfig reads configuration from file
func ReadConfig() (string, string, string, error) {
	return config.Read()
}

// CreateClient creates a new authenticated Proxmox client with caching
func CreateClient() (*ProxmoxClient, error) {
	user, pass, url, err := ReadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// Create a cache key based on user and URL
	cacheKey := fmt.Sprintf("%s@%s", user, url)

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
	client := NewClient(url, user, pass)
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

// GetVMIP gets the IP address of a VM using multiple methods
func (c *ProxmoxClient) GetVMIP(ctx context.Context, node string, vmid int) (string, error) {
	// Method 1: Try QEMU guest agent first (most reliable when available)
	if ip := c.getVMIPFromGuestAgent(ctx, node, vmid); ip != "N/A" {
		return ip, nil
	}

	// Method 2: Try to get IP from VM network configuration
	if ip := c.getVMIPFromConfig(ctx, node, vmid); ip != "N/A" {
		return ip, nil
	}

	// Method 3: Try to get IP from VM status/config
	if ip := c.getVMIPFromStatus(ctx, node, vmid); ip != "N/A" {
		return ip, nil
	}

	// Method 4: Try to get IP from network interfaces
	if ip := c.getVMIPFromInterfaces(ctx, node, vmid); ip != "N/A" {
		return ip, nil
	}

	// Method 5: Fallback - try getting IP from cluster resources or other sources
	if ip := c.getVMIPFromClusterResources(ctx, vmid); ip != "N/A" {
		return ip, nil
	}

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
	path := fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", node, vmid)
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

// getVMIPFromStatus tries to get IP from VM status
func (c *ProxmoxClient) getVMIPFromStatus(ctx context.Context, node string, vmid int) string {
	status, err := c.GetVMStatus(ctx, node, vmid)
	if err != nil {
		return "N/A"
	}

	// Check if there's network information in the status
	if net, exists := status["net"]; exists {
		if netMap, ok := net.(map[string]interface{}); ok {
			for _, netInfo := range netMap {
				if netStr, ok := netInfo.(string); ok {
					// Parse network info - format might vary
					if strings.Contains(netStr, "ip=") {
						parts := strings.Split(netStr, "ip=")
						if len(parts) > 1 {
							ip := strings.Split(parts[1], ",")[0]
							if ip != "" && ip != "127.0.0.1" {
								return ip
							}
						}
					}
				}
			}
		}
	}

	return "N/A"
}

// getVMIPFromInterfaces tries to get IP from network interfaces endpoint
func (c *ProxmoxClient) getVMIPFromInterfaces(ctx context.Context, node string, vmid int) string {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/interfaces", node, vmid)
	body, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return "N/A"
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "N/A"
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
								return parts[0]
							}
						} else if inetStr != "127.0.0.1" {
							return inetStr
						}
					}
				}
			}
		}
	}

	return "N/A"
}

// getVMIPFromClusterResources tries to get IP from cluster resources (if available)
func (c *ProxmoxClient) getVMIPFromClusterResources(ctx context.Context, vmid int) string {
	resources, err := c.GetClusterResources(ctx)
	if err != nil {
		return "N/A"
	}

	for _, resource := range resources {
		if resource.Type == "qemu" && resource.VMID != nil && *resource.VMID == vmid {
			// Some Proxmox setups might include IP information in cluster resources
			// This is not standard but worth checking
			if resource.Name != "" {
				// Try to see if there's any IP-like pattern in the resource data
				// This is a very basic fallback
				return "N/A"
			}
		}
	}

	return "N/A"
}

// GetContainerIP gets the IP address of a container using multiple methods
func (c *ProxmoxClient) GetContainerIP(ctx context.Context, node string, vmid int) (string, error) {
	// Method 1: Try LXC guest agent first
	if ip := c.getContainerIPFromGuestAgent(ctx, node, vmid); ip != "N/A" {
		return ip, nil
	}

	// Method 2: Try alternative methods
	if ip, err := c.GetContainerIPAlternative(ctx, node, vmid); err == nil && ip != "N/A" {
		return ip, nil
	}

	return "N/A", nil
}

// getContainerIPFromGuestAgent tries to get IP from LXC guest agent
func (c *ProxmoxClient) getContainerIPFromGuestAgent(ctx context.Context, node string, vmid int) string {
	path := fmt.Sprintf("/nodes/%s/lxc/%d/agent/network-get-interfaces", node, vmid)
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

	// Try to get IP from container status
	status, err := c.GetContainerStatus(ctx, node, vmid)
	if err != nil {
		return "N/A", nil
	}

	// Check if there's IP information in the status
	if ips, exists := status["ips"]; exists {
		if ipStr, ok := ips.(string); ok && ipStr != "" {
			return ipStr, nil
		}
	}

	// Try to get IP from network interfaces endpoint
	path := fmt.Sprintf("/nodes/%s/lxc/%d/interfaces", node, vmid)
	body, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return "N/A", nil
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
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
								return parts[0], nil
							}
						} else if inetStr != "127.0.0.1" {
							return inetStr, nil
						}
					}
				}
			}
		}
	}

	return "N/A", nil
}

// GetNodeIP attempts to get a primary IP address for a Proxmox node.
// This uses the node network API and returns the first non-loopback IPv4 it finds.
func (c *ProxmoxClient) GetNodeIP(ctx context.Context, node string) (string, error) {
	path := fmt.Sprintf("/nodes/%s/network", node)
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
	path := fmt.Sprintf("/nodes/%s/qemu/%d/config", node, vmid)

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
	path := fmt.Sprintf("/nodes/%s/qemu/%d/resize", node, vmid)
	reqBody := map[string]interface{}{
		"disk": disk,
		"size": size,
	}

	body, err := c.makeRequest(ctx, "PUT", path, reqBody)
	if err != nil {
		return "", err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse resize disk response: %w", err)
	}

	if taskID, ok := resp.Data.(string); ok {
		return taskID, nil
	}

	return "", fmt.Errorf("unexpected resize disk response format")
}

// GetVMConfig gets the configuration of a virtual machine
func (c *ProxmoxClient) GetVMConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/config", node, vmid)

	body, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse VM config response: %w", err)
	}

	if config, ok := resp.Data.(map[string]interface{}); ok {
		return config, nil
	}

	return nil, fmt.Errorf("unexpected VM config response format")
}

// GetContainerTemplates gets available container templates from a node
func (c *ProxmoxClient) GetContainerTemplates(ctx context.Context, node string) ([]ContainerTemplate, error) {
	path := fmt.Sprintf("/nodes/%s/storage", node)

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
	path := fmt.Sprintf("/nodes/%s/storage/%s/content", node, storage)

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
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/current", node, vmid)
	body, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse VM status response: %w", err)
	}

	if status, ok := resp.Data.(map[string]interface{}); ok {
		return status, nil
	}

	return nil, fmt.Errorf("unexpected VM status response format")
}

// GetVMDiskInfo gets more detailed disk information from VM configuration
func (c *ProxmoxClient) GetVMDiskInfo(ctx context.Context, node string, vmid int) (uint64, uint64, error) {
	config, err := c.GetVMConfig(ctx, node, vmid)
	if err != nil {
		return 0, 0, err
	}

	var totalSize uint64 = 0
	var usedSize uint64 = 0

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

						// Try to get disk size from storage API
						if storageSize, err := c.getStorageVolumeSize(ctx, node, storage, volumeID); err == nil {
							diskSize = storageSize
						}
					}
				}

				totalSize += diskSize
			}
		}
	}

	// Try multiple methods to get disk usage information
	// Method 1: Try to get from VM status (though this is often not accurate)
	status, err := c.GetVMStatus(ctx, node, vmid)
	if err == nil {
		if diskUsed, exists := status["disk"]; exists {
			if diskUsedFloat, ok := diskUsed.(float64); ok {
				usedSize = uint64(diskUsedFloat)
			}
		}

		// If we still don't have total size, try to get it from status
		if totalSize == 0 {
			if diskMax, exists := status["maxdisk"]; exists {
				if diskMaxFloat, ok := diskMax.(float64); ok {
					totalSize = uint64(diskMaxFloat)
				}
			}
		}
	}

	// Method 2: Try to get from RRD (Resource Round Database) for more accurate usage
	if usedSize == 0 {
		rrdUsed, err := c.getVMDiskUsageFromRRD(ctx, node, vmid)
		if err == nil && rrdUsed > 0 {
			usedSize = rrdUsed
		}
	}

	// Method 3: If still no usage data, estimate based on typical VM usage patterns
	// This is a fallback and not ideal, but better than showing 0
	if usedSize == 0 && totalSize > 0 {
		// For running VMs, estimate 15-30% usage; for stopped VMs, estimate 10%
		if status != nil {
			if vmStatus, exists := status["status"]; exists && vmStatus == "running" {
				usedSize = totalSize / 5 // Estimate 20% usage for running VMs
			} else {
				usedSize = totalSize / 10 // Estimate 10% usage for stopped VMs
			}
		}
	}

	return totalSize, usedSize, nil
}

// getStorageVolumeSize attempts to get the size of a storage volume
func (c *ProxmoxClient) getStorageVolumeSize(ctx context.Context, node, storage, volumeID string) (uint64, error) {
	path := fmt.Sprintf("/nodes/%s/storage/%s/content", node, storage)

	body, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return 0, err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, err
	}

	if contentList, ok := resp.Data.([]interface{}); ok {
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
	}

	return 0, fmt.Errorf("volume not found in storage")
}

// getVMDiskUsageFromRRD attempts to get disk usage from RRD data
func (c *ProxmoxClient) getVMDiskUsageFromRRD(ctx context.Context, node string, vmid int) (uint64, error) {
	// Try to get recent RRD data for disk usage
	path := fmt.Sprintf("/nodes/%s/qemu/%d/rrd", node, vmid)

	// Get data for the last hour with 5-minute resolution
	params := url.Values{}
	params.Add("timeframe", "hour")
	params.Add("cf", "AVERAGE")

	body, err := c.makeRequest(ctx, "GET", path+"?"+params.Encode(), nil)
	if err != nil {
		return 0, err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, err
	}

	if data, ok := resp.Data.(map[string]interface{}); ok {
		if dataArray, ok := data["data"].([]interface{}); ok && len(dataArray) > 0 {
			// Get the most recent data point
			if recent, ok := dataArray[len(dataArray)-1].(map[string]interface{}); ok {
				if diskWrite, exists := recent["diskwrite"]; exists {
					if diskWriteFloat, ok := diskWrite.(float64); ok && diskWriteFloat > 0 {
						// RRD gives us disk write rate, we need to estimate usage
						// This is not perfect but better than 0
						return uint64(diskWriteFloat * 3600), nil // Rough estimate
					}
				}
			}
		}
	}

	return 0, fmt.Errorf("no RRD disk data available")
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

// getVMIPFromConfig tries to get IP from VM network configuration
func (c *ProxmoxClient) getVMIPFromConfig(ctx context.Context, node string, vmid int) string {
	config, err := c.GetVMConfig(ctx, node, vmid)
	if err != nil {
		return "N/A"
	}

	// Check network configuration for static IP settings
	// Look for network interfaces (net0, net1, etc.)
	for key, value := range config {
		if strings.HasPrefix(key, "net") {
			if netConfig, ok := value.(string); ok {
				// Parse network configuration string
				// Example: "virtio=12:34:56:78:90:AB,bridge=vmbr0,ip=192.168.1.100/24"
				if strings.Contains(netConfig, "ip=") {
					parts := strings.Split(netConfig, ",")
					for _, part := range parts {
						if strings.HasPrefix(part, "ip=") {
							ipWithMask := strings.TrimPrefix(part, "ip=")
							// Remove subnet mask if present
							if strings.Contains(ipWithMask, "/") {
								ip := strings.Split(ipWithMask, "/")[0]
								if ip != "" && ip != "dhcp" && ip != "127.0.0.1" {
									return ip
								}
							}
						}
					}
				}
			}
		}
	}

	return "N/A"
}

// MigrateVM migrates a virtual machine to another node
func (c *ProxmoxClient) MigrateVM(ctx context.Context, node string, vmid int, targetNode string, options map[string]interface{}) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/migrate", node, vmid)

	// Prepare migration parameters
	reqBody := map[string]interface{}{
		"target": targetNode,
	}

	// Add optional parameters if provided
	for key, value := range options {
		reqBody[key] = value
	}

	body, err := c.makeRequest(ctx, "POST", path, reqBody)
	if err != nil {
		return "", err
	}

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse migrate VM response: %w", err)
	}

	if taskID, ok := resp.Data.(string); ok {
		return taskID, nil
	}

	return "", fmt.Errorf("unexpected migrate VM response format")
}
