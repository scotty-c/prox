# Prox Codebase Analysis Report

**Generated:** December 26, 2025
**Scope:** Code Quality, Performance, and UX Improvements

---

## Executive Summary

Prox is a well-architected CLI tool for Proxmox VE 8+ management with good separation of concerns, solid encryption practices, and thoughtful feature design. This analysis identifies opportunities for improvement across three categories:

- **Code Quality:** 23 items
- **Performance:** 12 items
- **UX Improvements:** 19 items

---

## Table of Contents

1. [Code Quality Improvements](#code-quality-improvements)
2. [Performance Optimizations](#performance-optimizations)
3. [UX Improvements](#ux-improvements)
4. [Priority Matrix](#priority-matrix)

---

## Code Quality Improvements

### High Priority

#### 1. Error Handling Inconsistency
**Location:** Multiple files
**Issue:** Functions inconsistently use `return` vs `os.Exit(1)` for error handling, making it difficult to use as a library and harder to test.

```go
// pkg/vm/operations.go:251-252 - Uses os.Exit
fmt.Printf("❌ Error creating client: %v\n", err)
os.Exit(1)

// pkg/vm/operations.go:51-52 - Uses return (better)
fmt.Printf("Error creating client: %v\n", err)
return
```

**Recommendation:** Standardize on returning errors from domain functions (`pkg/*`). Let the CLI layer (`cmd/*`) decide on exit behavior.

---

#### 2. Duplicate Code: IP Discovery Logic
**Location:** `pkg/client/client.go:711-794` and `pkg/client/client.go:900-983`
**Issue:** `getVMIPFromGuestAgent` and `getContainerIPFromGuestAgent` are nearly identical (~90% code duplication).

**Recommendation:** Extract common IP extraction logic into a shared helper:
```go
func extractIPFromGuestAgentResult(result map[string]interface{}) string {
    // Shared logic here
}
```

---

#### 3. Magic Numbers and Hardcoded Values
**Location:** Multiple files
**Issue:** Several magic numbers without constants:

| File | Line | Value | Purpose |
|------|------|-------|---------|
| `pkg/client/client.go` | 105 | `30 * time.Second` | HTTP timeout |
| `pkg/client/validation.go` | 14 | `100`, `999999999` | VMID range |
| `pkg/client/validation.go` | 27 | `15` | VM name max length |
| `pkg/vm/list.go` | 162 | `5`, `10` | Disk usage estimate divisors |

**Recommendation:** Move to `pkg/client/constants.go`:
```go
const (
    HTTPTimeout       = 30 * time.Second
    VMIDMin          = 100
    VMIDMax          = 999999999
    VMNameMaxLength  = 15
    // etc.
)
```

---

#### 4. Missing Error Wrapping Context
**Location:** `pkg/config/config.go:25-26`
**Issue:** Ignoring error from `os.UserHomeDir()`:
```go
home, _ := os.UserHomeDir()  // Error silently ignored
```

**Recommendation:** Handle the error or provide fallback:
```go
home, err := os.UserHomeDir()
if err != nil {
    return fmt.Errorf("failed to get home directory: %w", err)
}
```

---

#### 5. Typo in Code
**Location:** `pkg/config/config.go:12`
**Issue:** Comment says "Comfig" instead of "Config":
```go
// Comfig writes a local file to $HOME/.prox/config...
```

---

#### 6. Unused Function Pattern
**Location:** `cmd/ssh.go:191`
**Issue:** `findVMByNameOrID` and `findContainerByNameOrID` duplicate logic that exists in domain packages.

**Recommendation:** Move lookup functions to `pkg/vm` and `pkg/container` packages for reuse.

---

#### 7. Boolean Parameter Anti-pattern
**Location:** `pkg/vm/list.go:89`
```go
func ListVMs(node string, runningOnly bool, showIPs bool, detailed bool)
```

**Issue:** Multiple boolean parameters make calls confusing and error-prone.

**Recommendation:** Use options struct:
```go
type ListOptions struct {
    Node        string
    RunningOnly bool
    ShowIPs     bool
    Detailed    bool
}
func ListVMs(opts ListOptions) { ... }
```

---

### Medium Priority

#### 8. Context Not Passed Through
**Location:** Multiple locations
**Issue:** Functions create their own `context.Background()` instead of accepting context as parameter:

```go
// pkg/vm/list.go:23
resources, err := client.GetClusterResources(context.Background())
```

**Recommendation:** Pass context from CLI commands down through the stack for proper cancellation support.

---

#### 9. Inconsistent Naming
**Issue:** Mixed naming conventions:

| Current | Recommended |
|---------|-------------|
| `GetVm()` | `GetVM()` (Go convention for acronyms) |
| `ShutdownVm()` | `ShutdownVM()` |
| `StartVm()` | `StartVM()` |
| `StopVM()` (client) vs `ShutdownVm()` (vm) | Standardize on one term |

---

#### 10. Nil Pointer Risks
**Location:** `pkg/vm/list.go:38`
**Issue:** Potential nil pointer dereference if `resource.VMID` is nil:
```go
vm := VM{
    ID: int(*resource.VMID),  // Could panic if VMID is nil
```

**Recommendation:** Add nil checks:
```go
if resource.VMID == nil {
    continue
}
```

---

#### 11. File Path Construction
**Location:** `pkg/config/config.go`
**Issue:** Using string concatenation for paths:
```go
configFile := home + "/.prox/config"
```

**Recommendation:** Use `filepath.Join()` for cross-platform compatibility:
```go
configFile := filepath.Join(home, ".prox", "config")
```

---

#### 12. JSON Unmarshaling with Type Assertions
**Location:** `pkg/client/client.go:302-360`
**Issue:** Extensive manual type assertion instead of direct struct unmarshaling:
```go
if v, ok := resourceMap["id"].(string); ok {
    resource.ID = v
}
```

**Recommendation:** Use direct JSON unmarshaling into structs:
```go
type ResourcesResponse struct {
    Data []Resource `json:"data"`
}
var resp ResourcesResponse
json.Unmarshal(body, &resp)
```

---

### Low Priority

#### 13. Missing Package Documentation
**Issue:** Some packages lack package-level documentation comments.

**Affected Packages:**
- `pkg/conversion`
- `pkg/node`
- `cmd/node`

---

#### 14. Test File Organization
**Location:** `tests/` directory
**Issue:** Test files are outside the standard Go `*_test.go` convention.

**Recommendation:** Move unit tests adjacent to source files following Go conventions.

---

#### 15. Inconsistent Emoji Usage
**Issue:** Some messages use emojis, others don't:
```go
fmt.Printf("Error creating client: %v\n", err)      // No emoji
fmt.Printf("❌ Error getting cluster resources: %v\n", err)  // Has emoji
```

---

#### 16. Return Type Inconsistency
**Issue:** Some functions print and return nothing, others return errors:
```go
func GetVm() { ... }                    // Prints, returns nothing
func CreateContainer(...) error { ... } // Returns error
```

**Recommendation:** Standardize on returning errors from domain functions.

---

#### 17. Variable Shadowing
**Location:** `pkg/client/client.go:119-121`
```go
ciphertext_bytes := data[nonceSize:]  // Uses underscore (not Go style)
```

**Recommendation:** Use `ciphertextBytes` (camelCase).

---

#### 18. Redundant Code in Disk Estimation
**Location:** `pkg/vm/list.go:152-166`
**Issue:** Disk info is set twice with same values in some cases.

---

#### 19. Missing Validation Before Operations
**Location:** `pkg/vm/operations.go:148`
**Issue:** `CloneVm` validates new ID availability but not source VM existence first.

---

#### 20. Hardcoded Storage Reference
**Location:** `pkg/container/create.go:55`
```go
"rootfs": fmt.Sprintf("local-lvm:%d", disk),
```

**Recommendation:** Make storage configurable via flag or config.

---

#### 21. Missing Interface Definitions
**Issue:** No interfaces defined for testing/mocking the Proxmox client.

**Recommendation:**
```go
type ProxmoxClientInterface interface {
    GetClusterResources(ctx context.Context) ([]Resource, error)
    StartVM(ctx context.Context, node string, vmid int) (string, error)
    // ...
}
```

---

#### 22. Unused Error Return
**Location:** `pkg/container/templates.go:68`
**Issue:** `parseTemplateDescription` errors aren't captured.

---

#### 23. Command Alias Inconsistency
**Issue:** VM commands use `vm` prefix, container commands use both `ct` and `container`:
```
prox vm list
prox ct list        // Short
prox container list // Long (if exists?)
```

---

## Performance Optimizations

### High Priority

#### 1. Sequential IP Lookups
**Location:** `pkg/vm/list.go:65-74`
**Issue:** IP addresses are fetched sequentially for each running VM:
```go
for _, resource := range resources {
    // ...
    if resource.Status == "running" {
        ip, err := client.GetVMIP(context.Background(), resource.Node, int(*resource.VMID))
    }
}
```

**Impact:** N API calls for N running VMs, each taking ~100-500ms.

**Recommendation:** Use goroutines with a worker pool:
```go
const maxWorkers = 10
ipChan := make(chan ipResult, len(runningVMs))
sem := make(chan struct{}, maxWorkers)

for _, vm := range runningVMs {
    go func(vm VM) {
        sem <- struct{}{}
        defer func() { <-sem }()
        ip, _ := client.GetVMIP(ctx, vm.Node, vm.ID)
        ipChan <- ipResult{vm.ID, ip}
    }(vm)
}
```

**Estimated Improvement:** 5-10x faster for clusters with many VMs.

---

#### 2. Redundant API Calls
**Location:** `pkg/container/templates.go:104-158`
**Issue:** `ResolveTemplate` calls `getClusterNodes` and `getNodeTemplates` twice:
1. First to find a full-format template
2. Again for short-format resolution

**Recommendation:** Fetch templates once and search the cached results.

---

#### 3. No HTTP Connection Reuse
**Location:** `pkg/client/client.go:94-114`
**Issue:** `http.Client` is created with default settings that may not optimize connection reuse.

**Recommendation:** Configure transport for connection pooling:
```go
transport := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 10,
    IdleConnTimeout:     90 * time.Second,
    // ...
}
```

---

#### 4. Client Recreation Per Operation
**Location:** Multiple files
**Issue:** `CreateClient()` is called in almost every domain function, creating a new authenticated client each time:
```go
func ListVMs(...) {
    client, err := c.CreateClient()  // New client + auth
}
func StartVm(...) {
    client, err := c.CreateClient()  // Another new client + auth
}
```

**Recommendation:**
- Cache the authenticated client per session
- Add client reuse option for batch operations

---

### Medium Priority

#### 5. Disk Info Estimation Strategy
**Location:** `pkg/vm/list.go:160-166`
**Issue:** Disk usage is estimated with arbitrary ratios:
```go
vm.Disk = vm.MaxDisk / 5  // 20% for running
vm.Disk = vm.MaxDisk / 10 // 10% for stopped
```

**Recommendation:** Either:
- Remove estimation (show "N/A")
- Document estimation clearly in output
- Cache RRD data periodically for better estimates

---

#### 6. Template Search Efficiency
**Location:** `pkg/container/templates.go:170-182`
**Issue:** Linear search through all templates for matching:
```go
for _, template := range allTemplates {
    if templateOS == requestedOS || strings.Contains(...)
}
```

**Recommendation:** Build an index map for O(1) lookups:
```go
templateIndex := make(map[string][]Template)
// index by OS:version
```

---

#### 7. Task Polling Interval
**Location:** Various `waitForTask` implementations
**Issue:** Fixed polling intervals could be optimized.

**Recommendation:** Implement exponential backoff:
```go
interval := 500 * time.Millisecond
for {
    status, _ := client.GetTaskStatus(ctx, node, taskID)
    if status.Status == "stopped" {
        break
    }
    time.Sleep(interval)
    if interval < 5*time.Second {
        interval = interval * 3 / 2  // Exponential backoff
    }
}
```

---

#### 8. Cluster Resources Caching
**Issue:** `GetClusterResources()` is called frequently without caching.

**Recommendation:** Add short-lived cache (5-10 seconds):
```go
type ProxmoxClient struct {
    // ...
    resourcesCache    []Resource
    resourcesCacheExp time.Time
}

func (c *ProxmoxClient) GetClusterResources(ctx context.Context) ([]Resource, error) {
    if time.Now().Before(c.resourcesCacheExp) {
        return c.resourcesCache, nil
    }
    // Fetch fresh data...
}
```

---

### Low Priority

#### 9. String Building in Loops
**Location:** `cmd/ssh.go:329-345`
**Issue:** String concatenation in loop:
```go
newConfig.WriteString(line + "\n")
```

**Status:** Already using `strings.Builder` (good), but could pre-allocate capacity.

---

#### 10. Regex Compilation
**Location:** `pkg/client/validation.go:31`
**Issue:** Regex compiled on every validation call:
```go
matched, _ := regexp.MatchString("^[a-zA-Z0-9-]+$", name)
```

**Recommendation:** Compile once at package level:
```go
var vmNameRegex = regexp.MustCompile("^[a-zA-Z0-9-]+$")
```

---

#### 11. Unnecessary Allocations
**Location:** `pkg/client/client.go:258-280`
**Issue:** Creating new `Node{}` struct in loop when we could use pointers or pre-allocated slices.

---

#### 12. JSON Encoding Optimization
**Location:** `pkg/client/client.go:170-175`
**Issue:** `json.Marshal` called for every request body.

**Recommendation:** Use `json.NewEncoder` for streaming where applicable.

---

## UX Improvements

### High Priority

#### 1. Add `--json` Output Format
**Issue:** No machine-readable output option for scripting/automation.

**Recommendation:**
```bash
$ prox vm list --json
[{"id":100,"name":"web-server","status":"running",...}]
```

Implementation:
```go
listCmd.Flags().Bool("json", false, "Output in JSON format")
// In ListVMs:
if outputJSON {
    json.NewEncoder(os.Stdout).Encode(vms)
    return
}
```

---

#### 2. Add `--quiet` / `-q` Flag
**Issue:** No way to suppress progress messages for scripting:
```
$ prox vm start 100
Finding node for VM 100...     # Noise for scripts
Found VM 100 on node pve1
Starting VM 100 on node pve1...
VM 100 start command issued successfully
```

**Recommendation:**
```bash
$ prox vm start 100 -q
# (no output on success, only errors)
```

---

#### 3. Add Tab Completion
**Issue:** No shell completion support.

**Recommendation:** Cobra has built-in completion generation:
```go
var completionCmd = &cobra.Command{
    Use:   "completion [bash|zsh|fish|powershell]",
    Short: "Generate shell completion scripts",
}
```

---

#### 4. Add `prox vm/ct exec` Command
**Issue:** No way to execute commands inside VMs/containers directly.

**Recommendation:**
```bash
$ prox ct exec mycontainer -- ls -la
$ prox vm exec myvm -- cat /etc/hostname
```

---

#### 5. Missing `--force` Flag for Destructive Operations
**Location:** `prox vm delete`, `prox ct delete`
**Issue:** No confirmation prompt before deletion.

**Recommendation:**
```bash
$ prox vm delete 100
Are you sure you want to delete VM 100? [y/N]: y
VM 100 deleted.

$ prox vm delete 100 --force
VM 100 deleted.
```

---

#### 6. Add Progress Indicators for Long Operations
**Issue:** Long operations (clone, migrate) show no progress.

**Recommendation:** Use a progress bar or spinner:
```
$ prox vm clone 100 new-vm 101
Cloning VM... [===>    ] 45%
```

---

### Medium Priority

#### 7. Improve Error Messages
**Location:** Various
**Issue:** Some error messages are technical without actionable advice:
```
authentication failed with status 401: {"data":null}
```

**Recommendation:**
```
Authentication failed. Please check:
  - Username format: user@realm (e.g., admin@pam)
  - Password is correct
  - Proxmox URL is reachable

Run 'prox config setup' to reconfigure credentials.
```

---

#### 8. Add `prox status` Command
**Issue:** No quick way to check connection status and cluster health.

**Recommendation:**
```bash
$ prox status
Connected to: https://proxmox:8006
Cluster: homelab
Nodes: 3 (all online)
VMs: 15 (8 running)
Containers: 22 (18 running)
```

---

#### 9. Add VM/CT Name Support for All Commands
**Issue:** Some commands only accept VMID, not name:
```bash
$ prox vm start web-server  # Works
$ prox vm clone 100 ...     # ID only, should also accept name
```

---

#### 10. Add `prox logs` Command
**Issue:** No way to view Proxmox task logs from CLI.

**Recommendation:**
```bash
$ prox logs UPID:pve1:00001234:...
$ prox logs --follow  # Stream logs
```

---

#### 11. Add Config Validation Command
**Recommendation:**
```bash
$ prox config test
Testing connection to https://proxmox:8006...
Authentication: OK
API Version: 8.1.4
Node Access: OK (3 nodes)
Configuration is valid!
```

---

#### 12. Support for VM/CT Tags
**Issue:** No way to list/filter by Proxmox tags.

**Recommendation:**
```bash
$ prox vm list --tag production
$ prox ct list --tag web
```

---

#### 13. Reduce Emoji Usage for Professional Output
**Location:** All output messages across codebase
**Issue:** Heavy emoji usage makes output feel casual/unprofessional:
```
$ prox vm start 100
🔍 Finding node for VM 100...
📍 Found VM 100 on node pve1
🚀 Starting VM 100 on node pve1...
✅ VM 100 start command issued successfully
📋 Task ID: UPID:pve1:...
💡 Use 'prox vms list' to check the current status
```

**Recommendation:** Use minimal, purposeful indicators:
```
$ prox vm start 100
Finding node for VM 100...
Found VM 100 on node pve1
Starting VM 100 on node pve1...
VM 100 started successfully
Task ID: UPID:pve1:...

Tip: Use 'prox vm list' to check status
```

**Guidelines:**
- Remove emojis from standard output
- Use color (if terminal supports) for status: green=success, red=error, yellow=warning
- Keep output concise and scannable
- Reserve special characters for `--verbose` mode if needed
- Align with standard CLI tools (kubectl, docker, git)

**Files to update:**
- `pkg/vm/operations.go` - VM start/stop/clone/delete/migrate messages
- `pkg/vm/list.go` - VM listing output
- `pkg/container/create.go` - Container creation messages
- `pkg/container/operations.go` - Container start/stop/delete messages
- `pkg/container/templates.go` - Template listing
- `cmd/ssh.go` - SSH config messages
- `cmd/config/*.go` - Config setup messages
- `pkg/node/*.go` - Node operation messages

---

### Low Priority

#### 14. Add Version Command with Build Info
**Current:** No `prox version` command.

**Recommendation:**
```bash
$ prox version
prox version 1.2.3
Git commit: abc1234
Build date: 2025-01-15
Go version: 1.23.0
```

---

#### 15. Add Configuration Profiles
**Issue:** Only one config supported.

**Recommendation:**
```bash
$ prox config use production
$ prox --profile homelab vm list
```

---

#### 16. Interactive Mode
**Recommendation:**
```bash
$ prox interactive
prox> vm list
prox> ct start web-01
prox> exit
```

---

#### 17. Better Help Organization
**Issue:** Long help text could be better organized.

**Recommendation:** Group commands by category:
```
Virtual Machine Commands:
  vm list      List virtual machines
  vm start     Start a virtual machine
  ...

Container Commands:
  ct list      List containers
  ct create    Create a container
  ...
```

---

#### 18. Add `--wait` Flag
**Issue:** Some commands return immediately with task ID.

**Recommendation:**
```bash
$ prox vm clone 100 new-vm 101 --wait
Cloning... done (35s)
```

---

#### 19. Environment Variable Support
**Issue:** No support for environment variables for credentials.

**Recommendation:**
```bash
export PROX_URL=https://proxmox:8006
export PROX_USER=admin@pam
export PROX_PASSWORD=secret
prox vm list  # Uses env vars
```

---

## Priority Matrix

| Priority | Category | Item | Effort | Impact |
|----------|----------|------|--------|--------|
| **P0** | Performance | Parallel IP lookups | Medium | High |
| **P0** | UX | `--json` output | Low | High |
| **P0** | Code Quality | Standardize error handling | Medium | High |
| **P1** | Performance | Client caching | Medium | Medium |
| **P1** | UX | `--quiet` flag | Low | Medium |
| **P1** | UX | Remove emojis (professional output) | Medium | High |
| **P1** | Code Quality | Extract duplicate IP logic | Low | Medium |
| **P1** | UX | Shell completion | Low | Medium |
| **P2** | Performance | Template search optimization | Low | Low |
| **P2** | UX | `--force` confirmation | Low | Medium |
| **P2** | Code Quality | Options struct pattern | Medium | Low |
| **P3** | UX | Progress indicators | Medium | Medium |
| **P3** | Code Quality | Context propagation | High | Medium |
| **P3** | UX | Config profiles | High | Low |

---

## Recommended Next Steps

1. **Quick Wins (1-2 days):**
   - Add `--json` flag to list commands
   - Add `--quiet` flag
   - Fix typo in config.go
   - Extract constants from magic numbers
   - Add shell completion

2. **Medium Term (1-2 weeks):**
   - Remove emojis from all output for professional CLI experience
   - Implement parallel IP lookups
   - Add client caching
   - Standardize error handling pattern
   - Add `--force` and confirmation prompts
   - Add `prox status` command

3. **Long Term (1+ month):**
   - Add interfaces for testing
   - Implement progress indicators
   - Add configuration profiles
   - Add `prox exec` command
   - Full context propagation

---

## Appendix: Code Metrics

| Metric | Value |
|--------|-------|
| Total Go Files | 58 |
| Total Lines of Code | ~7,000 |
| Direct Dependencies | 2 (Cobra, go-pretty) |
| Test Coverage | Partial (E2E + unit) |
| Cyclomatic Complexity | Low-Medium |
| Package Count | 10 |

---

*Report generated by code analysis - December 2025*
