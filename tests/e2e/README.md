# Prox Comprehensive End-to-End Testing Framework

This directory contains a comprehensive end-to-end testing framework for the Prox CLI tool. The framework creates test resources, validates all functionality, and cleans up automatically.

## Overview

The comprehensive E2E testing framework provides:
- **Complete Resource Lifecycle Testing** - Creates, tests, and deletes VM and container resources
- **VM Cloning and Migration** - Tests VM cloning from existing VMs and migration between nodes  
- **Container Creation from Templates** - Tests container creation with SSH key support
- **SSH Configuration Testing** - Tests SSH config generation for VM and container access
- **Uses existing prox configuration** - No need to configure Proxmox connection details again
- **Full Operations Testing** - Comprehensive testing of all CLI operations on created resources
- **Migration testing** - Offline and online VM migration between nodes
- **Error handling** - Validates proper error responses for invalid operations
- **Automatic cleanup** - Removes all created test resources automatically
- **Detailed reporting** - Colored output and test report generation
- **Adaptive VM Shutdown** - Smart graceful→force escalation with guest-agent awareness
- **Transient Retry Logic** - Exponential backoff on known transient Proxmox/HTTP errors
- **Parallel Phases** - VM & container operation phases can run concurrently
- **Leftover Resource Handling** - Auto-clean or reuse resources from prior failed runs
- **Auto Binary Rebuild** - Rebuilds prox automatically if source files changed
- **Soft-Failure Polling** - Internal wait timeouts no longer inflate failure counts

## Quick Start

### 1. Setup Configuration

```bash
# Create configuration from example
./setup.sh setup

# Discover available resources for configuration
./setup.sh discover

# Edit the config file with your environment details
vi config.env

# Validate configuration
./setup.sh validate
```

### 2. Run Comprehensive Tests

```bash
# Recommended: Run all tests using configuration file
./setup.sh run

# Alternative: Run tests directly with parameters (only if you need to override config)
./run_e2e_tests.sh --vm-name e2e-test-vm --vm-id 9001 --ct-name e2e-test-ct --ct-id 9002 \
  --source-vm-id 100 --source-node node1 --target-node node2 \
  --ssh-key-file ~/.ssh/id_rsa.pub --verbose
```

## Files

- **`setup.sh`** - Configuration helper and test runner (**recommended way to run tests**)
- **`run_e2e_tests.sh`** - Main E2E testing script (can be run directly if needed)
- `config.env.example` - Example configuration file
- `config.env` - Your custom configuration (created by setup.sh)
- `README.md` - This documentation

## Quick Start Guide

**The simplest way to run E2E tests is using `setup.sh`:**

```bash
# 1. Setup configuration
./setup.sh setup
./setup.sh discover  # Optional: see available resources
./setup.sh validate

# 2. Run tests
./setup.sh run
```

## Configuration

### Required Parameters

The following parameters must be configured in `config.env` for comprehensive testing:

```bash
# Test VM (will be created by cloning existing VM)
TEST_VM_NAME="e2e-test-vm"        # Name for test VM that will be created
TEST_VM_ID="9001"                 # ID for test VM (must be available)
SOURCE_VM_ID="100"                # Existing VM to clone from (must exist)

# Test Container (will be created from template)
TEST_CT_NAME="e2e-test-ct"        # Name for test container that will be created
TEST_CT_ID="9002"                 # ID for test container (must be available)
TEST_TEMPLATE="ubuntu:22.04"      # Container template (see Template Format below)

# Node Configuration
SOURCE_NODE="proxmox01"           # Node where test resources will be created
TARGET_NODE="proxmox02"           # Target node for migration tests (optional)
```

### Template Format

The `TEST_TEMPLATE` parameter supports two formats:

1. **Short format** (recommended): `os:version`
   - Examples: `ubuntu:22.04`, `debian:12`, `alpine:3.18`
   - Prox will automatically discover the template location across all nodes

2. **Full format** (for exact control): `storage:vztmpl/template-name`
   - Example: `local:vztmpl/ubuntu-22.04-standard_22.04-1_amd64.tar.zst`
   - Use this when you need to specify the exact storage location

Use the full format if:
- Template exists on multiple storages and you need a specific one
- You want to ensure the template comes from a particular storage
- You're testing with custom or locally stored templates

**Tip**: Use `./setup.sh discover` to see all available templates and their exact storage locations.

**Usage Examples**:
```bash
# Using short format (auto-discovery)
export TEST_TEMPLATE="ubuntu:22.04"

# Using full format (exact storage control)
export TEST_TEMPLATE="local:vztmpl/ubuntu-22.04-standard_22.04-1_amd64.tar.zst"
```
```

### Optional Parameters

```bash
# SSH key for container creation tests
TEST_SSH_KEY_FILE="$HOME/.ssh/id_rsa.pub"

# Test behavior options
VERBOSE="false"
DRY_RUN="false"
SKIP_CLEANUP="false"              # Set to true to keep test resources for debugging

# Binary path (points to bin directory where make build creates the binary)
PROX_BINARY="../../bin/prox"

# Advanced / Feature Flags
WAIT_FOR_VM_IP="false"             # Require IP before VM marked ready (may slow tests)
RETRY_TRANSIENT="true"             # Retry transient API/agent failures
ENABLE_EXTRA_CONTAINER_TESTS="false" # Extra throwaway container lifecycle
AUTO_CLEAN_LEFTOVERS="false"      # Auto delete leftover test VM/CT if present
REUSE_LEFTOVERS="false"           # Reuse leftover VM/CT instead of recreating
AUTO_BUILD="true"                 # Rebuild prox if Go sources newer than binary
PARALLEL_VM_CT_TESTS="true"       # Run VM & container operation phases in parallel
```

## Test Categories

The comprehensive E2E tests follow a complete resource lifecycle with the following phases:

### 1. Configuration Management
- **Purpose**: Foundation testing - validates prox configuration
- **Tests**: Read current configuration, validate encrypted credential storage
- **Order**: First (prerequisite for all other tests)

### 2. Resource Creation
- **Purpose**: Create test resources for comprehensive testing
- **Tests**: 
  - Clone test VM from existing source VM
  - Create test container from specified template with SSH keys
  - Verify created resources are accessible
- **Order**: After configuration validation, before any operations

### 3. VM Operations (on created test VM)
- **Purpose**: Complete VM lifecycle testing
- **Tests**: 
  - List all VMs, describe test VM by ID/name
  - Edit VM configuration (safe modifications)
  - Power operations: Start → Verify → Stop → Verify
  - All operations performed on the newly created test VM
- **Order**: After VM creation, full lifecycle testing

### 4. Container Operations (on created test container)
- **Purpose**: Complete container lifecycle testing  
- **Tests**:
  - List containers, describe test container by ID/name
  - Template operations (list templates, shortcuts)
  - Power operations: Start → Verify → Stop → Verify
  - All operations performed on the newly created test container
- **Order**: After container creation, full lifecycle testing

### 5. VM Migration (Optional - requires target node)
- **Purpose**: Test advanced VM migration features
- **Tests**: 
  - Offline migration: Source → Target node
  - Online migration: Target → Source node (with running VM)
  - Verification of VM location after each migration
- **Order**: After basic VM operations, using the created test VM

### 6. Additional Container Creation (Optional - requires SSH key file)
- **Purpose**: Test SSH key validation and additional container features
- **Tests**: 
  - Create temporary container with SSH keys
  - Test operations on temporary container
  - Cleanup temporary container
- **Order**: After main container operations

### 7. SSH Key Validation (Optional - requires SSH key file)
- **Purpose**: Test SSH key format validation and edge cases
- **Tests**: Valid RSA/ED25519 keys, invalid formats, multiple keys, missing files
- **Order**: After container creation tests

### 8. SSH Configuration Testing
- **Purpose**: Test SSH configuration generation functionality
- **Tests**: 
  - Generate SSH config for VM by ID and name (dry-run)
  - Generate SSH config for container by ID and name (dry-run)
  - Test custom username, port, and key file options
  - Test error handling for non-existent resources
- **Order**: After SSH key validation, uses created test resources

### 9. Error Handling
- **Purpose**: Test error responses and edge cases
- **Tests**: Non-existent resources, invalid commands, invalid templates
- **Order**: After SSH configuration tests (uses invalid inputs that won't interfere with other tests)

### 10. Resource Cleanup
- **Purpose**: Remove all created test resources
- **Tests**: 
  - Stop and delete test VM
  - Stop and delete test container
  - Verify resources are removed
- **Order**: Final step (can be skipped with SKIP_CLEANUP=true for debugging)

## Test Flow Logic

The E2E testing framework follows these principles for reliable and realistic testing:

### Read-First Approach
- **List operations** are performed before describe operations
- **Describe operations** are performed before modification operations
- This ensures resources exist and are accessible before attempting changes

### Verification Steps
- **Power operations** include verification via describe commands
- Start VM → Describe to verify running state → Stop VM
- This confirms operations actually succeed, not just return success codes

### Lifecycle Testing
- **Container creation** tests include the full lifecycle:
  - Create → Describe → Start → Stop → Delete
- This ensures new resources work correctly through their entire lifecycle

### Non-Destructive Testing
- **Existing resources** (provided VM/container) are returned to original state
- **Temporary resources** (created containers) are fully cleaned up
- Tests can be run repeatedly without side effects

### Error Isolation
### Parallel Execution (Optional)
If `PARALLEL_VM_CT_TESTS=true` (default) the VM operations and container operations phases execute in parallel to reduce wall time. Results from both subshells are aggregated back into the main PASS/FAIL counters.

Disable with:
```bash
export PARALLEL_VM_CT_TESTS=false
```

### Leftover Resource Handling
On startup the script detects if the intended test VM / container IDs already exist (from a prior aborted run). Behavior:

1. If `AUTO_CLEAN_LEFTOVERS=true`: shutdown & delete them automatically.
2. Else if `REUSE_LEFTOVERS=true`: mark them for reuse (skip creation).
3. Else (interactive TTY): prompt to Delete / Reuse / Abort.
4. Else (non-interactive without flags): fail fast with guidance.

If both AUTO_CLEAN_LEFTOVERS and REUSE_LEFTOVERS are true, auto-clean wins.

### Adaptive VM Shutdown
`safe_vm_shutdown` strategy:
1. Graceful shutdown (retry if enabled).
2. Poll for stopped state (shorter timeout if guest agent absent).
3. Multiple force-stop attempts with unlock waits.
4. Final graceful attempt.
Intermediate polling timeouts are warnings (non-fatal); only final failure counts as test failure.

### Transient Retry Logic
When `RETRY_TRANSIENT=true`, operations prone to temporary errors (guest agent ping, locks, 5xx) use exponential backoff (`retry_run`). This reduces flakiness in noisy clusters.

### Soft Failures
Internal wait loops (status/lock polling) and explicitly soft-marked commands can emit `[WARN]` instead of `[FAIL]`, preventing expected timing windows from skewing statistics.

### Auto Build
If `AUTO_BUILD=true` the script re-builds the prox binary when Go sources are newer than the existing binary. Disable with `AUTO_BUILD=false` once stable.

### Wait for VM IP
Enable `WAIT_FOR_VM_IP=true` to require an IP address before considering a cloned VM ready (useful for tests depending on networking). May increase runtime.
- **Error handling tests** use invalid inputs that won't affect real resources
- **Expected failures** are clearly marked and validated
- This prevents false positives from error conditions

## Usage Examples

### Basic Testing
```bash
# Test with minimal required parameters
./run_e2e_tests.sh \
  --vm-name production-vm \
  --vm-id 100 \
  --ct-name web-container \
  --ct-id 200
```

### Comprehensive Testing
```bash
# Test with all features
./run_e2e_tests.sh \
  --vm-name test-vm \
  --vm-id 100 \
  --ct-name test-ct \
  --ct-id 200 \
  --ssh-key-file ~/.ssh/id_rsa.pub \
  --target-node node2 \
  --verbose
```

### Development Testing
```bash
# Dry run to see what tests would execute
./run_e2e_tests.sh \
  --vm-name test-vm \
  --vm-id 100 \
  --ct-name test-ct \
  --ct-id 200 \
  --dry-run

# Skip cleanup for debugging
./run_e2e_tests.sh \
  --vm-name test-vm \
  --vm-id 100 \
  --ct-name test-ct \
  --ct-id 200 \
  --skip-cleanup
```

### Using Configuration File
```bash
# Setup and configure
./setup.sh setup
./setup.sh validate

# Run tests
./setup.sh run

# Check current configuration
./setup.sh config
```

## Prerequisites

### Environment Requirements
1. **Proxmox VE Server** - Accessible Proxmox VE 8+ environment
2. **Prox Configuration** - Run `prox config setup` first (E2E tests use this existing configuration)
3. **Test Resources** - Existing VM and container for testing
4. **Permissions** - Appropriate user permissions for all operations

### Test Resources
The test VM and container must:
- Exist in your Proxmox environment
- Be accessible with your current credentials
- Support start/stop operations
- Allow configuration changes

### Optional Requirements
- **SSH Key File** - For container creation tests
- **Multiple Nodes** - For migration testing
- **Template Access** - For container creation tests

## Test Output

### Successful Run
```
[INFO] Starting E2E tests with:
[INFO]   VM: test-vm (ID: 100)
[INFO]   Container: test-ct (ID: 200)
[INFO]   SSH Key File: /home/user/.ssh/id_rsa.pub

[PASS] Read current configuration
[PASS] List all VMs
[PASS] Describe VM by ID (100)
[PASS] Start VM (100)
[PASS] Shutdown VM (100)
...

==================================
End-to-End Test Summary
==================================
Tests Passed: 15
Tests Failed: 0
Total Tests: 15

All tests passed! 🎉
```

### Failed Tests
```
[FAIL] Start VM (100)
Command: ./prox vm start 100
Output: Error: VM not found
Exit code: 1

==================================
End-to-End Test Summary
==================================
Tests Passed: 12
Tests Failed: 3
Total Tests: 15

Failed Tests:
  ✗ Start VM (100)
  ✗ Describe non-existent VM
  ✗ Invalid template handling
```

## Test Reports

The framework generates detailed test reports:
- **Timestamped filename** - `e2e_test_report_YYYYMMDD_HHMMSS.txt`
- **Configuration details** - Test parameters and environment
- **Complete results** - Pass/fail status for all tests
- **Failure details** - Command output and error information

## Troubleshooting

### Common Issues

**"Prox is not configured"**
```bash
# Configure prox first (E2E tests will use this configuration)
prox config setup -u admin@pam -p password -l https://proxmox.example.com:8006
```

**"Test VM/container not found"**
- Verify the VM/container exists in Proxmox
- Check that IDs and names match exactly
- Ensure proper permissions

**"SSH key file not found"**
- Verify the SSH key file path
- Generate SSH key if needed: `ssh-keygen -t ed25519`

**"Target node not available"**
- Verify target node exists in cluster
- Check node is online and accessible
- Ensure migration permissions

### Debug Mode

Enable verbose output for detailed debugging:
```bash
./run_e2e_tests.sh --verbose [other options]
```

Use dry run to see what would be executed:
```bash
./run_e2e_tests.sh --dry-run [other options]
```

## Best Practices
## Environment Variable Summary

| Variable | Default | Description |
|----------|---------|-------------|
| TEST_VM_NAME / TEST_VM_ID | (required) | Name/ID for test VM clone target |
| SOURCE_VM_ID | (required) | Existing VM ID used as clone source |
| TEST_CT_NAME / TEST_CT_ID | (required) | Name/ID for test container |
| TEST_TEMPLATE | ubuntu:22.04 | Container template (short or full format) |
| SOURCE_NODE | (required) | Node for creating resources |
| TARGET_NODE | (optional) | Node for migration tests |
| TEST_SSH_KEY_FILE | (optional) | SSH public key for container setup |
| VERBOSE | false | Verbose logging |
| DRY_RUN | false | Show commands without executing |
| SKIP_CLEANUP | false | Keep resources after run |
| PROX_BINARY | ../../bin/prox | Path to prox binary |
| WAIT_FOR_VM_IP | false | Require IP before VM readiness |
| RETRY_TRANSIENT | true | Enable transient retry wrapper |
| ENABLE_EXTRA_CONTAINER_TESTS | false | Extra temp container lifecycle |
| AUTO_CLEAN_LEFTOVERS | false | Auto delete leftover test resources |
| REUSE_LEFTOVERS | false | Reuse leftover resources |
| AUTO_BUILD | true | Auto rebuild binary if sources newer |
| PARALLEL_VM_CT_TESTS | true | Parallelize VM & container phases |

Note: If both AUTO_CLEAN_LEFTOVERS and REUSE_LEFTOVERS are true, AUTO_CLEAN_LEFTOVERS has precedence.

### Test Environment
- Use dedicated test VMs/containers when possible
- Backup important VMs before testing
- Test in non-production environments first
- Verify cluster health before migration tests

### CI/CD Integration
The E2E tests can be integrated into CI/CD pipelines:
```bash
# Example CI script
./setup.sh setup
# Configure environment variables
./setup.sh validate
./setup.sh run
```

### Regular Testing
Run E2E tests regularly to:
- Validate new Prox versions
- Test against new Proxmox releases
- Verify environment changes
- Catch regressions early

## Contributing

When adding new tests:
1. Follow the existing test pattern
2. Add appropriate error handling
3. Include both positive and negative test cases
4. Update documentation
5. Test with various configurations

## Support

For issues with the E2E testing framework:
1. Check this README for common solutions
2. Review test output and logs
3. Verify Proxmox environment health
4. Open an issue with detailed error information
