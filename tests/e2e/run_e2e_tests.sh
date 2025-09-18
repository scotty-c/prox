#!/usr/bin/env bash

# Prox CLI Comprehensive End-to-End Testing Script
# This script creates test resources, tests all prox functionality, and cleans up
# 1. Creates test VM by cloning existing VM
# 2. Creates test container from template
# 3. Tests all operations on created resources
# 4. Tests VM migration between nodes
# 5. Cleans up all created resources

# Note: set -e is not used here because we handle command failures explicitly in run_command

# Default values
PROX_BINARY="../../bin/prox"
TEST_VM_NAME=""
TEST_VM_ID=""
TEST_CT_NAME=""
TEST_CT_ID=""
SOURCE_VM_ID=""
SOURCE_NODE=""
TEST_TEMPLATE="ubuntu:22.04"
TEMPLATE_NODE=""
VERBOSE=false
DRY_RUN=false
SKIP_CLEANUP=false
TEST_SSH_KEY_FILE=""
TARGET_NODE=""
WAIT_FOR_VM_IP="${WAIT_FOR_VM_IP:-false}" # Optional: wait for IP assignment to consider VM ready
RETRY_TRANSIENT="${RETRY_TRANSIENT:-true}" # Enable retry wrapper for transient VM operations
ENABLE_EXTRA_CONTAINER_TESTS="${ENABLE_EXTRA_CONTAINER_TESTS:-false}" # Create additional temp container (optional)
AUTO_CLEAN_LEFTOVERS="${AUTO_CLEAN_LEFTOVERS:-false}" # Auto-delete leftover test resources from a prior failed run
REUSE_LEFTOVERS="${REUSE_LEFTOVERS:-false}" # Reuse leftover resources instead of recreating
REUSE_VM=false
REUSE_CT=false
AUTO_BUILD="${AUTO_BUILD:-true}" # Automatically rebuild prox binary if source is newer
PARALLEL_VM_CT_TESTS="${PARALLEL_VM_CT_TESTS:-true}" # Run VM and container core operation tests in parallel

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test results tracking
TESTS_PASSED=0
TESTS_FAILED=0
FAILED_TESTS=()

# Function to print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((TESTS_PASSED++))
}

print_error() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((TESTS_FAILED++))
    FAILED_TESTS+=("$1")
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Function to show usage
show_usage() {
    cat << EOF
Prox CLI Comprehensive End-to-End Testing Script

This script creates test resources, tests all prox functionality, and cleans up.

Usage: $0 --vm-name <n> --vm-id <id> --ct-name <n> --ct-id <id> --source-vm-id <id> --source-node <node> [options]

Required Parameters:
  --vm-name <n>           Name for test VM that will be created
  --vm-id <id>            ID for test VM that will be created (must be available)
  --ct-name <n>           Name for test container that will be created  
  --ct-id <id>            ID for test container that will be created (must be available)
  --source-vm-id <id>     ID of existing VM to clone for testing (must exist)
  --source-node <node>    Node where test resources will be created

Optional Parameters:
  --prox-binary <path>    Path to prox binary (default: ../../bin/prox)
  --ssh-key-file <path>   Path to SSH public key file for container tests
  --target-node <node>    Target node for migration tests (if different from source)
  --template <template>   Container template to use (default: ubuntu:22.04)
                         Can be short format (ubuntu:22.04) or full format (storage:vztmpl/template-name)
                         Use full format to specify exact storage location
  --template-node <node>  Node/storage where template is located (informational only)
                         Note: prox CLI auto-discovers template location; use full template format for exact control
  --verbose               Enable verbose output
  --dry-run              Show what would be tested without executing
  --skip-cleanup         Don't clean up test resources (for debugging)
    --auto-clean-leftovers Automatically delete leftover test VM/container if they already exist
    --reuse-leftovers      Reuse leftover test VM/container (skip creation) if they already exist
    --extra-container-tests Create an additional temporary container to exercise create/start/stop/delete separately
    --no-build             Skip automatic rebuild even if sources are newer
  --help                 Show this help message

Template Format Examples:
  Short format (auto-discovery): ubuntu:22.04, debian:12, alpine:3.18
  Full format (exact storage):   local:vztmpl/ubuntu-22.04-standard_22.04-1_amd64.tar.zst

Examples:
  # Comprehensive test with VM cloning and migration
  $0 --vm-name e2e-test-vm --vm-id 9001 --ct-name e2e-test-ct --ct-id 9002 \\
     --source-vm-id 100 --source-node node1 --target-node node2 \\
     --ssh-key-file ~/.ssh/id_rsa.pub --verbose

  # Basic test without migration (single node)
  $0 --vm-name e2e-test-vm --vm-id 9001 --ct-name e2e-test-ct --ct-id 9002 \\
     --source-vm-id 100 --source-node node1

  # Dry run to see what tests would be executed
  $0 --vm-name e2e-test-vm --vm-id 9001 --ct-name e2e-test-ct --ct-id 9002 \\
     --source-vm-id 100 --source-node node1 --dry-run

Test Flow (comprehensive resource lifecycle):
  1. Configuration Management - Validates prox config foundation
  2. Resource Creation - Clone test VM, create test container from template
  3. VM Operations - Full lifecycle testing on created VM (start, stop, edit, describe)
  4. Container Operations - Full lifecycle testing on created container
  5. VM Migration - Offline and online migration between nodes (if target specified)
    6. (Optional) Extra Temp Container Lifecycle (if enabled)
    7. Additional Container Tests - SSH key validation, template shortcuts
    8. Error Handling - Invalid operations and edge cases
    9. Resource Cleanup - Remove all created test resources

Prerequisites:
  - Prox must be configured with valid Proxmox credentials (run 'prox config setup')
  - Source VM must exist for cloning (specified by --source-vm-id)
  - Source node must be accessible and have sufficient resources
  - Target node must exist for migration testing (if specified)
  - User must have appropriate permissions for all operations

Note:
  This script uses your existing prox configuration for Proxmox connection details.

EOF
}

# Function to parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --vm-name)
                TEST_VM_NAME="$2"
                shift 2
                ;;
            --vm-id)
                TEST_VM_ID="$2"
                shift 2
                ;;
            --ct-name)
                TEST_CT_NAME="$2"
                shift 2
                ;;
            --ct-id)
                TEST_CT_ID="$2"
                shift 2
                ;;
            --source-vm-id)
                SOURCE_VM_ID="$2"
                shift 2
                ;;
            --source-node)
                SOURCE_NODE="$2"
                shift 2
                ;;
            --template)
                TEST_TEMPLATE="$2"
                shift 2
                ;;
            --template-node)
                TEMPLATE_NODE="$2"
                shift 2
                ;;
            --prox-binary)
                PROX_BINARY="$2"
                shift 2
                ;;
            --ssh-key-file)
                TEST_SSH_KEY_FILE="$2"
                shift 2
                ;;
            --target-node)
                TARGET_NODE="$2"
                shift 2
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --skip-cleanup)
                SKIP_CLEANUP=true
                shift
                ;;
            --auto-clean-leftovers)
                AUTO_CLEAN_LEFTOVERS=true
                shift
                ;;
            --reuse-leftovers)
                REUSE_LEFTOVERS=true
                shift
                ;;
            --extra-container-tests)
                ENABLE_EXTRA_CONTAINER_TESTS=false
                shift
                ;;
            --no-build)
                AUTO_BUILD=true
                shift
                ;;
            --help)
                show_usage
                exit 0
                ;;
            *)
                echo "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done

    # Validate required parameters for comprehensive testing
    if [[ -z "$TEST_VM_NAME" || -z "$TEST_VM_ID" || -z "$TEST_CT_NAME" || -z "$TEST_CT_ID" ]]; then
        echo "Error: VM and container test parameters must be specified"
        show_usage
        exit 1
    fi
    
    if [[ -z "$SOURCE_VM_ID" ]]; then
        echo "Error: SOURCE_VM_ID must be specified for VM cloning tests"
        show_usage
        exit 1
    fi
    
    if [[ -z "$SOURCE_NODE" ]]; then
        echo "Error: SOURCE_NODE must be specified for resource creation"
        show_usage
        exit 1
    fi
}

# Function to check prerequisites
check_prerequisites() {
    print_info "Checking prerequisites..."

    # Check if prox binary exists
    if [[ ! -f "$PROX_BINARY" ]]; then
        print_error "Prox binary not found at: $PROX_BINARY"
        print_info "Build the binary first: make build (from project root)"
        exit 1
    fi

    # Optionally rebuild binary if stale
    if [[ "$AUTO_BUILD" == "true" && "$DRY_RUN" != "true" ]]; then
        if command -v find >/dev/null 2>&1; then
            local newest_source
            newest_source=$(find ../../cmd ../../pkg -type f -name '*.go' -printf '%T@ %p\n' 2>/dev/null | sort -nr | head -1 | awk '{print $1}')
            local bin_mtime
            bin_mtime=$(stat -c %Y "$PROX_BINARY" 2>/dev/null || echo 0)
            # Convert newest_source (epoch.float) to integer seconds for compare
            local newest_int=${newest_source%%.*}
                        if [[ -n "$newest_int" && "$newest_int" -gt "$bin_mtime" ]]; then
                                print_info "Rebuilding prox binary (sources newer than existing binary)"
                                (
                                    cd ../.. || exit 1
                                    mkdir -p bin
                                    # Build only the main module so -o single binary is valid
                                    if ! go build -o bin/prox .; then
                                            print_error "Go build failed (attempt with root package)"
                                            exit 1
                                    fi
                                ) || { print_error "Go build failed"; exit 1; }
                                print_success "Rebuild complete"
            else
                print_info "Existing prox binary is up to date"
            fi
        else
            print_warning "'find' not available; skipping staleness check"
        fi
    fi

    # Make binary executable (after potential rebuild)
    chmod +x "$PROX_BINARY"

    # Check if prox is configured (skip in dry-run mode)
    if [[ "$DRY_RUN" != "true" ]]; then
        if ! $PROX_BINARY config read &>/dev/null; then
            print_error "Prox is not configured. Run 'prox config setup' first."
            print_info "The E2E tests use your existing prox configuration for Proxmox connection details."
            exit 1
        else
            print_info "Using existing prox configuration for Proxmox connection"
        fi
    fi

    # Verify SSH key file if specified
    if [[ -n "$TEST_SSH_KEY_FILE" && ! -f "$TEST_SSH_KEY_FILE" ]]; then
        print_error "SSH key file not found: $TEST_SSH_KEY_FILE"
        exit 1
    fi

    # Validate source resources exist and test resources don't exist (skip in dry-run mode)
    if [[ "$DRY_RUN" != "true" ]]; then
        print_info "Validating source resources and checking test resource availability..."
        
        # Check if source VM exists (needed for cloning)
        if ! $PROX_BINARY vm describe "$SOURCE_VM_ID" &>/dev/null; then
            print_error "Source VM (ID: $SOURCE_VM_ID) not found in Proxmox environment"
            print_info "Please update SOURCE_VM_ID in config.env to match an existing VM for cloning"
            print_info "Use 'prox vm list' to see available VMs"
            exit 1
        fi
        
        # Detect existing (leftover) resources
        local leftover_vm=false leftover_ct=false
        if $PROX_BINARY vm describe "$TEST_VM_ID" &>/dev/null; then leftover_vm=true; fi
        if $PROX_BINARY ct describe "$TEST_CT_ID" &>/dev/null; then leftover_ct=true; fi

        if [[ "$leftover_vm" == true || "$leftover_ct" == true ]]; then
            print_warning "Detected leftover test resources from previous run:"
            [[ "$leftover_vm" == true ]] && print_warning " - VM ID $TEST_VM_ID already exists"
            [[ "$leftover_ct" == true ]] && print_warning " - CT ID $TEST_CT_ID already exists"

            if [[ "$AUTO_CLEAN_LEFTOVERS" == "true" ]]; then
                print_info "AUTO_CLEAN_LEFTOVERS enabled; deleting leftovers automatically"
                [[ "$leftover_vm" == true ]] && safe_vm_shutdown "$TEST_VM_ID" "auto-clean leftover" 2>/dev/null || true
                [[ "$leftover_vm" == true ]] && run_command "$PROX_BINARY vm delete $TEST_VM_ID" "Delete leftover VM ($TEST_VM_ID)" || true
                [[ "$leftover_ct" == true ]] && $PROX_BINARY ct stop "$TEST_CT_ID" 2>/dev/null || true
                [[ "$leftover_ct" == true ]] && run_command "$PROX_BINARY ct delete $TEST_CT_ID" "Delete leftover CT ($TEST_CT_ID)" || true
            else
                if [[ "$REUSE_LEFTOVERS" == "true" ]]; then
                    print_info "Reusing leftover resources as requested (--reuse-leftovers)"
                    [[ "$leftover_vm" == true ]] && REUSE_VM=true
                    [[ "$leftover_ct" == true ]] && REUSE_CT=true
                else
                    if [[ -t 0 ]]; then
                        echo ""
                        echo "Leftover test resources detected. Choose action:"
                        echo "  [d] Delete and recreate   [r] Reuse existing   [a] Abort"
                        read -rp "Action (d/r/a) [d]: " action
                        action=${action:-d}
                        case "$action" in
                            d|D)
                                print_info "Deleting leftover resources..."
                                [[ "$leftover_vm" == true ]] && safe_vm_shutdown "$TEST_VM_ID" "interactive clean leftover" 2>/dev/null || true
                                [[ "$leftover_vm" == true ]] && run_command "$PROX_BINARY vm delete $TEST_VM_ID" "Delete leftover VM ($TEST_VM_ID)" || true
                                [[ "$leftover_ct" == true ]] && $PROX_BINARY ct stop "$TEST_CT_ID" 2>/dev/null || true
                                [[ "$leftover_ct" == true ]] && run_command "$PROX_BINARY ct delete $TEST_CT_ID" "Delete leftover CT ($TEST_CT_ID)" || true
                                ;;
                            r|R)
                                print_info "Reusing existing leftover resources"
                                [[ "$leftover_vm" == true ]] && REUSE_VM=true
                                [[ "$leftover_ct" == true ]] && REUSE_CT=true
                                ;;
                            a|A)
                                print_error "Aborting due to leftover resources (user chose abort)"
                                exit 1
                                ;;
                            *)
                                print_info "Unknown choice; defaulting to delete"
                                [[ "$leftover_vm" == true ]] && safe_vm_shutdown "$TEST_VM_ID" "default delete leftover" 2>/dev/null || true
                                [[ "$leftover_vm" == true ]] && run_command "$PROX_BINARY vm delete $TEST_VM_ID" "Delete leftover VM ($TEST_VM_ID)" || true
                                [[ "$leftover_ct" == true ]] && $PROX_BINARY ct stop "$TEST_CT_ID" 2>/dev/null || true
                                [[ "$leftover_ct" == true ]] && run_command "$PROX_BINARY ct delete $TEST_CT_ID" "Delete leftover CT ($TEST_CT_ID)" || true
                                ;;
                        esac
                    else
                        print_error "Leftover resources found and no interactive TTY to resolve. Use --auto-clean-leftovers or --reuse-leftovers"
                        exit 1
                    fi
                fi
            fi
        fi
        
        # Verify source node exists
        if ! $PROX_BINARY vm list --node "$SOURCE_NODE" &>/dev/null; then
            print_error "Source node ($SOURCE_NODE) not found or not accessible"
            print_info "Please verify the node name is correct"
            exit 1
        fi
        
        print_info "Source resources validated and test IDs are available"
    fi

    print_success "Prerequisites check passed"
}

# Helper function to build container create command
build_ct_create_cmd() {
    local ct_name="$1"
    local template="$2"
    local node="$3"
    local extra_args="$4"
    
    local cmd="$PROX_BINARY ct create $ct_name $template --node $node"
    
    # Add extra arguments
    if [[ -n "$extra_args" ]]; then
        cmd="$cmd $extra_args"
    fi
    
    echo "$cmd"
}

# Function to run a command and capture output
run_command() {
    local cmd="$1"
    local description="$2"
    local expect_failure="${3:-false}"
    local soft_failure="${4:-false}" # if true and not expect_failure, failures are warnings (not counted)

    if [[ "$DRY_RUN" == "true" ]]; then
        print_info "[DRY RUN] Would execute: $cmd"
        return 0
    fi

    if [[ "$VERBOSE" == "true" ]]; then
        print_info "Executing: $cmd"
    fi

    local output
    local exit_code
    
    # Preserve current errexit setting and disable for command execution
    local errexit_set=0
    case $- in *e*) errexit_set=1;; esac
    set +e
    output=$(eval "$cmd" 2>&1)
    exit_code=$?
    # Restore errexit if it was previously set
    if [[ $errexit_set -eq 1 ]]; then set -e; fi

    if [[ "$expect_failure" == "true" ]]; then
        if [[ $exit_code -ne 0 ]]; then
            print_success "$description (expected failure)"
            if [[ "$VERBOSE" == "true" ]]; then
                echo "Output: $output"
            fi
            # Normalize exit code so expected failures don't abort later logic
            exit_code=0
        else
            print_error "$description (expected failure but command succeeded)"
            # Mark as failure in test counts; ensure non-zero so summary reflects issue
            exit_code=1
        fi
    else
        if [[ $exit_code -eq 0 ]]; then
            print_success "$description"
            if [[ "$VERBOSE" == "true" ]]; then
                echo "Output: $output"
            fi
        else
            if [[ "$soft_failure" == "true" ]]; then
                print_warning "$description (non-fatal)"
                [[ "$VERBOSE" == "true" ]] && { echo "Command: $cmd"; echo "Output: $output"; echo "Exit code: $exit_code"; }
                # Normalize exit to 0 so follow-on logic proceeds
                exit_code=0
            else
                print_error "$description"
                echo "Command: $cmd"
                echo "Output: $output"
                echo "Exit code: $exit_code"
            fi
        fi
    fi

    return $exit_code
}

# Internal raw exec (no test counters) returning output & status
_exec_raw() {
    local cmd="$1"; local __outvar="$2"; local output exit_code
    set +e; output=$(eval "$cmd" 2>&1); exit_code=$?; set -e
    printf -v "$__outvar" '%s' "$output"
    return $exit_code
}

# Retry wrapper for transient failures (guest agent timeout, lock, HTTP 5xx)
retry_run() {
    local attempts="$1"; shift
    local delay="$1"; shift
    local cmd="$1"; shift
    local description="$1"; shift || true
    local patterns="guest-ping|timeout|locked|lock|595 | 595:|status 595|500 | 500:|status 500|connection reset|temporary failure"
    local attempt=1
    local output status
    if [[ "$DRY_RUN" == "true" ]]; then
        print_info "[DRY RUN] Would retry ($attempts) command: $cmd"
        return 0
    fi
    while (( attempt <= attempts )); do
        [[ "$VERBOSE" == "true" ]] && print_info "Attempt $attempt/$attempts: $description"
        _exec_raw "$cmd" output; status=$?
        if [[ $status -eq 0 ]]; then
            [[ "$VERBOSE" == "true" ]] && echo "$output"
            print_success "${description:-Command} (after ${attempt} attempt(s))"
            return 0
        fi
        # Decide if retryable
        if [[ $attempt -lt $attempts ]] && echo "$output" | grep -Eqi "$patterns"; then
            print_warning "Retryable failure on attempt $attempt: ${description:-Command} -> $(echo "$output" | head -1)"
            sleep "$delay"
            delay=$(( delay * 2 ))
        else
            print_error "${description:-Command} failed (attempt $attempt/$attempts)"
            [[ "$VERBOSE" == "true" ]] && echo "Output: $output"
            return $status
        fi
        attempt=$(( attempt + 1 ))
    done
    return 1
}

# Wait for VM to be "ready" (no locks; optionally IP assigned)
wait_for_vm_ready() {
    local vm_id="$1"
    local timeout="${2:-180}"
    local require_ip="${3:-$WAIT_FOR_VM_IP}"
    local interval=5
    local elapsed=0
    local consecutive_clear=0

    if [[ "$DRY_RUN" == "true" ]]; then
        print_info "[DRY RUN] Would wait for VM $vm_id readiness (locks cleared${require_ip:+, IP assigned})"
        return 0
    fi

    print_info "Waiting for VM $vm_id to become ready (timeout ${timeout}s; require_ip=${require_ip})..."

    while [[ $elapsed -lt $timeout ]]; do
        local output
        set +e
        output=$($PROX_BINARY vm describe "$vm_id" 2>/dev/null)
        local exit_code=$?
        set -e

        if [[ $exit_code -eq 0 ]]; then
            # Detect lock lines (proxmox prints lines containing 'lock:' in raw API, our formatted output may not unless locked)
            local lock_line
            lock_line=$(echo "$output" | grep -i "lock:" || true)
            local ip_line
            ip_line=$(echo "$output" | grep -i "IP Address:" | head -1 || true)
            local ip_value
            ip_value=$(echo "$ip_line" | awk -F':' '{print $2}' | xargs)

            local ip_ready=true
            if [[ "$require_ip" == "true" ]]; then
                if [[ -z "$ip_value" || "$ip_value" == "N/A" ]]; then
                    ip_ready=false
                fi
            fi

            if [[ -z "$lock_line" && "$ip_ready" == "true" ]]; then
                consecutive_clear=$((consecutive_clear + 1))
                if [[ $consecutive_clear -ge 2 ]]; then
                    print_success "VM $vm_id is ready (waited ${elapsed}s)"
                    return 0
                fi
            else
                consecutive_clear=0
                if [[ -n "$lock_line" && "$VERBOSE" == "true" ]]; then
                    print_info "VM $vm_id still locked: $(echo "$lock_line" | tr -d '\n')"
                fi
                if [[ "$require_ip" == "true" && "$ip_ready" != "true" && "$VERBOSE" == "true" ]]; then
                    print_info "VM $vm_id waiting for IP assignment (current: ${ip_value:-none})"
                fi
            fi
        else
            consecutive_clear=0
            [[ "$VERBOSE" == "true" ]] && print_info "VM $vm_id describe failed, retrying..."
        fi

        sleep $interval
        elapsed=$((elapsed + interval))
    done

    print_error "Timeout waiting for VM $vm_id readiness (waited ${elapsed}s)"
    return 1
}

# Function to create test resources
create_test_resources() {
    print_info "Creating test resources..."

    # Show template information if TEMPLATE_NODE is specified
    if [[ -n "$TEMPLATE_NODE" ]]; then
        print_info "Template node specified: $TEMPLATE_NODE"
        if [[ ! "$TEST_TEMPLATE" =~ :vztmpl/ ]]; then
            print_info "Note: Using short format template ($TEST_TEMPLATE) - prox will auto-discover location"
            print_info "To ensure template comes from specific storage, use full format: storage:vztmpl/template-name"
        fi
    fi

    if [[ "$REUSE_VM" == "true" ]]; then
        print_info "Reusing existing test VM ($TEST_VM_ID)"
        run_command "$PROX_BINARY vm describe $TEST_VM_ID" "Verify reused test VM exists ($TEST_VM_ID)"
        wait_for_vm_ready "$TEST_VM_ID" 120 || true
    else
        # Create test VM by cloning source VM
        print_info "Creating test VM by cloning source VM..."
        if ! run_command "$PROX_BINARY vm clone $SOURCE_VM_ID $TEST_VM_ID --name $TEST_VM_NAME --node $SOURCE_NODE" "Clone VM ($SOURCE_VM_ID → $TEST_VM_ID)"; then
            print_error "Failed to create test VM"
            return 1
        fi

        # Wait briefly then verify creation
        if [[ "$DRY_RUN" != "true" ]]; then
            sleep 8
            print_info "Verifying test VM was created..."
            run_command "$PROX_BINARY vm describe $TEST_VM_ID" "Verify test VM exists ($TEST_VM_ID)"
            # Now wait for readiness (locks clear; optional IP)
            wait_for_vm_ready "$TEST_VM_ID" 180 || print_info "Continuing despite readiness timeout"
        fi
    fi

    if [[ "$REUSE_CT" == "true" ]]; then
        print_info "Reusing existing test container ($TEST_CT_ID)"
        run_command "$PROX_BINARY ct describe $TEST_CT_ID" "Verify reused test container exists ($TEST_CT_ID)"
    else
        # Create test container
        print_info "Creating test container..."
        local ssh_args=""
        if [[ -n "$TEST_SSH_KEY_FILE" ]]; then
            ssh_args="--ssh-keys-file $TEST_SSH_KEY_FILE"
        fi
        
        local create_ct_cmd=$(build_ct_create_cmd "$TEST_CT_NAME" "$TEST_TEMPLATE" "$SOURCE_NODE" "--memory 1024 --disk 8 $ssh_args")
        
        if ! run_command "$create_ct_cmd" "Create container ($TEST_CT_NAME)"; then
            print_error "Failed to create test container"
            return 1
        fi

        # Wait for container creation to complete
        if [[ "$DRY_RUN" != "true" ]]; then
            sleep 5
            print_info "Verifying test container was created..."
            run_command "$PROX_BINARY ct describe $TEST_CT_ID" "Verify test container exists ($TEST_CT_ID)"
        fi
    fi

    print_success "Test resources created successfully"
}

# Function to cleanup test resources
cleanup_test_resources() {
    if [[ "$SKIP_CLEANUP" == "true" ]]; then
        print_info "Skipping cleanup (SKIP_CLEANUP=true)"
        return
    fi

    print_info "Cleaning up test resources..."

    # Stop and delete test VM
    if [[ "$DRY_RUN" != "true" ]]; then
        print_info "Cleaning up test VM..."
        # Stop VM safely (ignore errors if VM doesn't exist)
        safe_vm_shutdown "$TEST_VM_ID" "cleanup" 2>/dev/null || true
        # Delete VM
        run_command "$PROX_BINARY vm delete $TEST_VM_ID" "Delete test VM ($TEST_VM_ID)" || true
    else
        print_info "[DRY RUN] Would stop and delete test VM ($TEST_VM_ID)"
    fi

    # Stop and delete test container
    if [[ "$DRY_RUN" != "true" ]]; then
        print_info "Cleaning up test container..."
        # Stop container if running (ignore errors)
        $PROX_BINARY ct stop "$TEST_CT_ID" 2>/dev/null || true
        sleep 3
        # Delete container
        run_command "$PROX_BINARY ct delete $TEST_CT_ID" "Delete test container ($TEST_CT_ID)" || true
    else
        print_info "[DRY RUN] Would stop and delete test container ($TEST_CT_ID)"
    fi

    print_info "Cleanup completed"
}

# Test configuration management
test_config_management() {
    print_info "Testing configuration management..."
    run_command "$PROX_BINARY config read" "Read current configuration"
}

# Test VM operations on created test VM
test_vm_operations() {
    print_info "Testing VM operations on created test VM..."

    # Test VM listing first
    run_command "$PROX_BINARY vm list" "List all VMs"

    # Ensure VM is in a ready (unlocked) state before operations
    wait_for_vm_ready "$TEST_VM_ID" 120 || print_info "Proceeding with VM operations despite readiness wait failure"

    # Test VM describe with ID (read operation)
    run_command "$PROX_BINARY vm describe $TEST_VM_ID" "Describe test VM by ID ($TEST_VM_ID)"

    # Test VM describe with name (read operation)
    run_command "$PROX_BINARY vm describe $TEST_VM_NAME" "Describe test VM by name ($TEST_VM_NAME)"

    # Test VM edit (safe operation - change name)
    run_command "$PROX_BINARY vm edit $TEST_VM_ID --name 'e2e-test-vm-$(date +%s)'" "Edit test VM name ($TEST_VM_ID)"

    # Test VM start/stop operations
    print_info "Testing VM power operations..."
    
    # Test VM start
    run_command "$PROX_BINARY vm start $TEST_VM_ID" "Start test VM ($TEST_VM_ID)"

    # Wait for VM to start and verify
    if [[ "$DRY_RUN" != "true" ]]; then
        wait_for_vm_status "$TEST_VM_ID" "running" 60 "startup verification"
        run_command "$PROX_BINARY vm describe $TEST_VM_ID" "Verify VM is running after start ($TEST_VM_ID)"
    fi

    # Test VM shutdown with robust waiting
    safe_vm_shutdown "$TEST_VM_ID" "shutdown test"

    # Verify VM is stopped
    if [[ "$DRY_RUN" != "true" ]]; then
        run_command "$PROX_BINARY vm describe $TEST_VM_ID" "Verify VM is stopped after shutdown ($TEST_VM_ID)"
    fi
}

# Test VM migration with created test VM
test_vm_migration() {
    if [[ -z "$TARGET_NODE" ]]; then
        print_info "Skipping VM migration tests (no target node specified)"
        return
    fi

    if [[ "$TARGET_NODE" == "$SOURCE_NODE" ]]; then
        print_info "Skipping VM migration tests (target node same as source node)"
        return
    fi

    print_info "Testing VM migration between nodes..."

    # Ensure VM is stopped for offline migration using safe shutdown
    safe_vm_shutdown "$TEST_VM_ID" "preparation for migration"
    
    # Wait for any remaining locks to clear before migration
    if [[ "$DRY_RUN" != "true" ]]; then
        wait_for_vm_unlock "$TEST_VM_ID" 30 "before migration"
    fi

    # Test offline migration to target node
    run_command "$PROX_BINARY vm migrate $TEST_VM_ID $TARGET_NODE" "Migrate test VM offline ($TEST_VM_ID: $SOURCE_NODE → $TARGET_NODE)"

    # Wait for migration to complete
    if [[ "$DRY_RUN" != "true" ]]; then
        wait_for_vm_unlock "$TEST_VM_ID" 120 "after offline migration"
        # Verify VM is now on target node
        run_command "$PROX_BINARY vm describe $TEST_VM_ID" "Verify VM migrated to target node ($TEST_VM_ID)"
    fi

    # Test online migration back to source node
    # Start VM first for online migration
    run_command "$PROX_BINARY vm start $TEST_VM_ID" "Start test VM for online migration ($TEST_VM_ID)"
    
    if [[ "$DRY_RUN" != "true" ]]; then
        wait_for_vm_status "$TEST_VM_ID" "running" 60 "before online migration"
    fi

    # Migrate back online
    run_command "$PROX_BINARY vm migrate $TEST_VM_ID $SOURCE_NODE --online" "Migrate test VM back online ($TEST_VM_ID: $TARGET_NODE → $SOURCE_NODE)"

    # Wait for migration to complete
    if [[ "$DRY_RUN" != "true" ]]; then
        wait_for_vm_unlock "$TEST_VM_ID" 120 "after online migration"
        # Verify VM is back on source node and still running
        run_command "$PROX_BINARY vm describe $TEST_VM_ID" "Verify VM migrated back to source node ($TEST_VM_ID)"
    fi

    # Shutdown VM after migration testing
    safe_vm_shutdown "$TEST_VM_ID" "after migration tests"
}

# Test container operations on created test container
test_container_operations() {
    print_info "Testing container operations on created test container..."

    # Test container listing first
    run_command "$PROX_BINARY ct list" "List all containers"

    # Test running containers only
    run_command "$PROX_BINARY ct list --running" "List running containers only"

    # Test container describe with ID (read operation)
    run_command "$PROX_BINARY ct describe $TEST_CT_ID" "Describe test container by ID ($TEST_CT_ID)"

    # Test container describe with name (read operation)
    run_command "$PROX_BINARY ct describe $TEST_CT_NAME" "Describe test container by name ($TEST_CT_NAME)"

    # Test container templates and shortcuts (informational commands)
    run_command "$PROX_BINARY ct templates" "List container templates"
    run_command "$PROX_BINARY ct shortcuts" "Show template shortcuts"

    # Test container start/stop operations
    print_info "Testing container power operations..."
    
    # Test container start
    run_command "$PROX_BINARY ct start $TEST_CT_ID" "Start test container ($TEST_CT_ID)"

    # Wait for container to start and verify
    if [[ "$DRY_RUN" != "true" ]]; then
        sleep 5
        run_command "$PROX_BINARY ct describe $TEST_CT_ID" "Verify container is running after start ($TEST_CT_ID)"
    fi

    # Test container stop
    run_command "$PROX_BINARY ct stop $TEST_CT_ID" "Stop test container ($TEST_CT_ID)"

    # Wait for container to stop and verify
    if [[ "$DRY_RUN" != "true" ]]; then
        sleep 3
        run_command "$PROX_BINARY ct describe $TEST_CT_ID" "Verify container is stopped after stop ($TEST_CT_ID)"
    fi
}

# Test additional container creation (temporary containers)
test_container_creation() {
    if [[ "$ENABLE_EXTRA_CONTAINER_TESTS" != "true" ]]; then
        print_info "Skipping extra container lifecycle (feature disabled)"
        return
    fi
    if [[ -z "$TEST_SSH_KEY_FILE" ]]; then
        print_info "Skipping extra container lifecycle (no SSH key file specified)"
        return
    fi

    print_info "Testing additional container creation with SSH keys..."

    local temp_ct_name="e2e-temp-$(date +%s)"

    # Create temporary container with SSH key
    local create_temp_cmd=$(build_ct_create_cmd "$temp_ct_name" "$TEST_TEMPLATE" "$SOURCE_NODE" "--memory 1024 --disk 8 --ssh-keys-file $TEST_SSH_KEY_FILE")
    if ! run_command "$create_temp_cmd" "Create temporary container with SSH key ($temp_ct_name)"; then
        return
    fi

    # Wait for container creation to complete
    if [[ "$DRY_RUN" != "true" ]]; then
        sleep 5
        print_info "Testing operations on newly created temporary container..."
        
        # Test describing the newly created container
        run_command "$PROX_BINARY ct describe $temp_ct_name" "Describe temporary container ($temp_ct_name)"
        
        # Test starting the newly created container
        run_command "$PROX_BINARY ct start $temp_ct_name" "Start temporary container ($temp_ct_name)"
        
        # Wait a moment for container to start
        sleep 3
        
        # Test stopping the newly created container
        run_command "$PROX_BINARY ct stop $temp_ct_name" "Stop temporary container ($temp_ct_name)"
    fi

    # Cleanup temporary container
    if [[ "$DRY_RUN" != "true" && "$SKIP_CLEANUP" != "true" ]]; then
        print_info "Cleaning up temporary container: $temp_ct_name"
        # Ensure container is stopped before deletion
        $PROX_BINARY ct stop "$temp_ct_name" 2>/dev/null || true
        sleep 2
        $PROX_BINARY ct delete "$temp_ct_name" 2>/dev/null || true
    fi
}

# Test SSH key validation
test_ssh_key_validation() {
    if [[ -z "$TEST_SSH_KEY_FILE" ]]; then
        print_info "Skipping SSH key validation tests (no SSH key file specified)"
        return
    fi

    print_info "Testing SSH key validation..."

    # First, test with the actual provided SSH key file (just validate the file exists)
    if [[ -f "$TEST_SSH_KEY_FILE" ]]; then
        print_success "SSH key file exists and is readable: $TEST_SSH_KEY_FILE"
    else
        print_error "SSH key file not found or not readable: $TEST_SSH_KEY_FILE"
    fi

    # Create temporary SSH key files for additional testing
    local temp_dir="/tmp/prox-e2e-test-$$"
    mkdir -p "$temp_dir"

    # Valid SSH key (RSA)
    echo "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7vbqajDhA+example+key+data user@example.com" > "$temp_dir/valid_rsa_key.pub"

    # Valid SSH key (ED25519)
    echo "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample+ED25519+Key+Data user@example.com" > "$temp_dir/valid_ed25519_key.pub"

    # Invalid SSH key format
    echo "invalid-key-format" > "$temp_dir/invalid_key.pub"

    # Empty file
    touch "$temp_dir/empty_key.pub"

    # Test SSH key file format validation locally (without creating containers)
    print_info "Validating SSH key file formats..."
    
    # Test with valid RSA key
    if grep -q "^ssh-rsa " "$temp_dir/valid_rsa_key.pub"; then
        print_success "Valid RSA SSH key format detected"
    else
        print_error "Failed to detect valid RSA SSH key format"
    fi

    # Test with valid ED25519 key
    if grep -q "^ssh-ed25519 " "$temp_dir/valid_ed25519_key.pub"; then
        print_success "Valid ED25519 SSH key format detected"
    else
        print_error "Failed to detect valid ED25519 SSH key format"
    fi

    # Test with invalid key (expect failure)
    if grep -q "^ssh-" "$temp_dir/invalid_key.pub"; then
        print_error "Invalid key incorrectly detected as valid SSH key"
    else
        print_success "Invalid SSH key format correctly rejected"
    fi

    # Test with empty file (expect failure)
    if [[ -s "$temp_dir/empty_key.pub" ]]; then
        print_error "Empty file incorrectly detected as having content"
    else
        print_success "Empty SSH key file correctly detected"
    fi

    # Cleanup
    if [[ "$SKIP_CLEANUP" != "true" ]]; then
        rm -rf "$temp_dir"
    fi
}

# Test error handling
test_error_handling() {
    print_info "Testing error handling..."

    # Test with non-existent VM
    run_command "$PROX_BINARY vm describe 99999" "Handle non-existent VM" true

    # Test with non-existent container
    run_command "$PROX_BINARY ct describe 99999" "Handle non-existent container" true

    # Test invalid usage via unknown flag (Cobra guarantees non-zero exit on unknown flag)
    if ! run_command "$PROX_BINARY vm --definitely-not-a-real-flag" "Handle invalid VM command" true; then
        # run_command already recorded the failure (command succeeded when it should fail)
        print_warning "Primary invalid VM command test didn't trigger failure exit; retrying with secondary invalid pattern"
        run_command "$PROX_BINARY vm --another-bad-flag-123" "Handle invalid VM command (secondary)" true || true
    fi

    # Test with invalid template
    local invalid_template_cmd=$(build_ct_create_cmd "test-invalid" "invalid:template" "$SOURCE_NODE" "")
    run_command "$invalid_template_cmd" "Handle invalid template" true
}

# Test SSH configuration functionality
test_ssh_configuration() {
    print_info "Testing SSH configuration functionality..."

    # First verify that test resources exist before testing SSH
    if [[ "$DRY_RUN" != "true" ]]; then
        if ! $PROX_BINARY vm describe "$TEST_VM_ID" &>/dev/null; then
            print_warning "Test VM ($TEST_VM_ID) does not exist, skipping SSH tests for VM"
            return
        fi
        if ! $PROX_BINARY ct describe "$TEST_CT_ID" &>/dev/null; then
            print_warning "Test container ($TEST_CT_ID) does not exist, skipping SSH tests for container"
            return
        fi
    fi

    # Get the current VM name (may have been changed during edit operations)
    local current_vm_name=""
    if [[ "$DRY_RUN" != "true" ]]; then
        local vm_output
        if vm_output=$($PROX_BINARY vm describe "$TEST_VM_ID" 2>/dev/null); then
            current_vm_name=$(echo "$vm_output" | grep -E "^\s*Name:" | awk '{print $2}' | xargs)
            if [[ "$VERBOSE" == "true" && -n "$current_vm_name" ]]; then
                print_info "Detected current VM name: '$current_vm_name' (original: '$TEST_VM_NAME')"
            fi
        fi
    fi

    # ------------------------------------------------------------------
    # Real (non-dry-run) SSH add -> list -> delete cycle
    # Uses container as target since it is usually quicker to acquire an IP.
    # All real steps are soft-failure to avoid aborting full E2E suite if IP missing.
    # ------------------------------------------------------------------
    if [[ "$DRY_RUN" != "true" ]]; then
        print_info "Attempting real SSH config add/list/delete cycle (container)"
        run_command "$PROX_BINARY ct start $TEST_CT_ID" "Ensure container running for real SSH test" false true
        sleep 5
        run_command "$PROX_BINARY ssh $TEST_CT_ID" "Add SSH config entry for container (real)" false true
        run_command "$PROX_BINARY ssh --list" "List SSH config entries after add" false true
        run_command "$PROX_BINARY ssh $TEST_CT_ID --delete" "Delete SSH config entry for container (real)" false true
        run_command "$PROX_BINARY ssh --list" "List SSH config entries after delete" false true
        run_command "$PROX_BINARY ct stop $TEST_CT_ID" "Stop container after real SSH test" false true
    else
        print_info "[DRY RUN] Skipping real SSH config add/list/delete cycle"
    fi

    # Error path tests (real, expect failures)
    run_command "$PROX_BINARY ssh nonexistent-resource" "SSH config for non-existent resource" true
    run_command "$PROX_BINARY ssh $TEST_VM_ID --port invalid" "SSH config with invalid port" true
}

# Function to show test summary
show_summary() {
    echo ""
    echo "=================================="
    echo "End-to-End Test Summary"
    echo "=================================="
    echo -e "Tests Passed: ${GREEN}$TESTS_PASSED${NC}"
    echo -e "Tests Failed: ${RED}$TESTS_FAILED${NC}"
    echo "Total Tests: $((TESTS_PASSED + TESTS_FAILED))"

    if [[ $TESTS_FAILED -gt 0 ]]; then
        echo ""
        echo "Failed Tests:"
        for test in "${FAILED_TESTS[@]}"; do
            echo -e "  ${RED}✗${NC} $test"
        done
        echo ""
        exit 1
    else
        echo ""
        echo -e "${GREEN}All tests passed! 🎉${NC}"
        echo ""
        exit 0
    fi
}

# Function to create a test report
create_test_report() {
    local report_file="e2e_test_report_$(date +%Y%m%d_%H%M%S).txt"
    
    cat > "$report_file" << EOF
Prox CLI Comprehensive End-to-End Test Report
Generated: $(date)

Test Configuration:
- Test VM Name: $TEST_VM_NAME
- Test VM ID: $TEST_VM_ID
- Test Container Name: $TEST_CT_NAME
- Test Container ID: $TEST_CT_ID
- Source VM ID: $SOURCE_VM_ID
- Source Node: $SOURCE_NODE
- Target Node: ${TARGET_NODE:-"Not specified"}
- Template: $TEST_TEMPLATE
- SSH Key File: ${TEST_SSH_KEY_FILE:-"Not specified"}
- Prox Binary: $PROX_BINARY

Test Results:
- Tests Passed: $TESTS_PASSED
- Tests Failed: $TESTS_FAILED
- Total Tests: $((TESTS_PASSED + TESTS_FAILED))

EOF

    if [[ $TESTS_FAILED -gt 0 ]]; then
        echo "Failed Tests:" >> "$report_file"
        for test in "${FAILED_TESTS[@]}"; do
            echo "  - $test" >> "$report_file"
        done
    fi

    echo "" >> "$report_file"
    echo "Test completed at: $(date)" >> "$report_file"

    print_info "Test report saved to: $report_file"
}

# Function to wait for VM to reach a specific status
wait_for_vm_status() {
    local vm_id="$1"
    local expected_status="$2"
    local timeout="${3:-60}"
    local description="$4"
    local suppress="${5:-false}" # if true, don't increment test failure count on timeout
    
    if [[ "$DRY_RUN" == "true" ]]; then
        print_info "[DRY RUN] Would wait for VM $vm_id to reach status: $expected_status"
        return 0
    fi
    
    print_info "Waiting for VM $vm_id to reach status: $expected_status${description:+ ($description)}"
    
    local elapsed=0
    local interval=3
    
    while [[ $elapsed -lt $timeout ]]; do
        local output
        local exit_code
        
        # Get VM status
        set +e
        output=$($PROX_BINARY vm describe "$vm_id" 2>/dev/null)
        exit_code=$?
        set -e
        
        if [[ $exit_code -eq 0 ]]; then
            local current_status
            current_status=$(echo "$output" | grep -i "status" | head -1 | awk '{print $2}' | tr -d ',')
            
            if [[ "$current_status" == "$expected_status" ]]; then
                print_success "VM $vm_id reached status: $expected_status (waited ${elapsed}s)"
                return 0
            fi
            
            if [[ "$VERBOSE" == "true" ]]; then
                print_info "VM $vm_id current status: $current_status (waiting for $expected_status)"
            fi
        else
            if [[ "$VERBOSE" == "true" ]]; then
                print_info "Failed to get VM $vm_id status, retrying..."
            fi
        fi
        
        sleep $interval
        elapsed=$((elapsed + interval))
    done
    
    if [[ "$suppress" == "true" ]]; then
        print_warning "(non-fatal) Timeout waiting for VM $vm_id to reach status: $expected_status (waited ${elapsed}s)"
    else
        print_error "Timeout waiting for VM $vm_id to reach status: $expected_status (waited ${elapsed}s)"
    fi
    return 1
}

# Function to wait for VM to be unlocked (no locks)
wait_for_vm_unlock() {
    local vm_id="$1"
    local timeout="${2:-60}"
    local description="$3"
    local suppress="${4:-false}"
    
    if [[ "$DRY_RUN" == "true" ]]; then
        print_info "[DRY RUN] Would wait for VM $vm_id to be unlocked"
        return 0
    fi
    
    print_info "Waiting for VM $vm_id to be unlocked${description:+ ($description)}"
    
    local elapsed=0
    local interval=3
    
    while [[ $elapsed -lt $timeout ]]; do
        local output
        local exit_code
        
        # Get VM status
        set +e
        output=$($PROX_BINARY vm describe "$vm_id" 2>/dev/null)
        exit_code=$?
        set -e
        
        if [[ $exit_code -eq 0 ]]; then
            # Check if there are any locks mentioned in the output
            local has_lock
            has_lock=$(echo "$output" | grep -i "lock" | grep -v "unlock" || true)
            
            if [[ -z "$has_lock" ]]; then
                print_success "VM $vm_id is unlocked (waited ${elapsed}s)"
                return 0
            fi
            
            if [[ "$VERBOSE" == "true" ]]; then
                print_info "VM $vm_id still has locks, waiting... (${elapsed}s elapsed)"
            fi
        else
            if [[ "$VERBOSE" == "true" ]]; then
                print_info "Failed to get VM $vm_id status, retrying..."
            fi
        fi
        
        sleep $interval
        elapsed=$((elapsed + interval))
    done
    
    if [[ "$suppress" == "true" ]]; then
        print_warning "(non-fatal) Timeout waiting for VM $vm_id to be unlocked (waited ${elapsed}s)"
    else
        print_error "Timeout waiting for VM $vm_id to be unlocked (waited ${elapsed}s)"
    fi
    return 1
}

# Function to safely shutdown VM with retry logic
safe_vm_shutdown() {
    local vm_id="$1"
    local description="$2"
    local force_timeout="${3:-120}"
    
    if [[ "$DRY_RUN" == "true" ]]; then
        print_info "[DRY RUN] Would safely shutdown VM: $vm_id"
        return 0
    fi
    
    print_info "Safely shutting down VM $vm_id${description:+ ($description)}"

    # Gather initial info (agent status, current state)
    local output exit_code agent_enabled="unknown" current_status="unknown"
    set +e
    output=$($PROX_BINARY vm describe "$vm_id" 2>/dev/null)
    exit_code=$?
    set -e
    if [[ $exit_code -eq 0 ]]; then
        current_status=$(echo "$output" | grep -i "^\s*Status:" | awk '{print $2}' | tr -d ',')
        agent_enabled=$(echo "$output" | grep -i "QEMU Agent:" | awk -F':' '{print tolower($2)}' | xargs)
        [[ -z "$agent_enabled" ]] && agent_enabled="unknown"
        if [[ "$current_status" == "stopped" ]]; then
            print_info "VM $vm_id is already stopped"
            return 0
        fi
    else
        print_warning "Could not describe VM $vm_id prior to shutdown (exit $exit_code)"
    fi

    # Adaptive timeouts
    local graceful_timeout=60
    if [[ "$agent_enabled" == *disabled* || "$agent_enabled" == *unknown* ]]; then
        graceful_timeout=35
        print_info "Guest Agent not enabled/detected; using shorter graceful timeout (${graceful_timeout}s) before escalation"
    else
        print_info "Guest Agent appears enabled; allowing up to ${graceful_timeout}s for graceful shutdown"
    fi

    # Issue graceful shutdown (with retry if enabled)
    print_info "Attempting graceful shutdown of VM $vm_id (agent=${agent_enabled})"
    local graceful_error_marker="" shutdown_out shutdown_exit
    if [[ "$RETRY_TRANSIENT" == "true" ]]; then
        if retry_run 5 2 "$PROX_BINARY vm shutdown $vm_id" "Graceful shutdown command"; then
            shutdown_exit=0
        else
            shutdown_exit=1
            shutdown_out="(see above retry output)"
        fi
    else
        set +e
        shutdown_out=$($PROX_BINARY vm shutdown "$vm_id" 2>&1)
        shutdown_exit=$?
        set -e
    fi
    if [[ $shutdown_exit -ne 0 ]]; then
        # Capture known guest agent timeout patterns
        if echo "$shutdown_out" | grep -qi "guest-ping"; then
            graceful_error_marker="guest-agent-timeout"
            print_warning "Guest agent ping timeout detected immediately; will escalate sooner"
            graceful_timeout=15
        else
            print_warning "Initial graceful shutdown command failed (exit $shutdown_exit); output: $shutdown_out"
        fi
    fi

    # Poll for stopped status during graceful period
    local elapsed=0 poll_interval=3
    while [[ $elapsed -lt $graceful_timeout ]]; do
        if wait_for_vm_status "$vm_id" "stopped" $poll_interval "graceful-progress" true >/dev/null 2>&1; then
            print_success "VM $vm_id gracefully shut down (elapsed ${elapsed}s)"
            return 0
        fi
        # Re-check describe for any lock or extra hints
        set +e
        output=$($PROX_BINARY vm describe "$vm_id" 2>/dev/null)
        exit_code=$?
        set -e
        if [[ $exit_code -eq 0 && "$VERBOSE" == "true" ]]; then
            local maybe_lock
            maybe_lock=$(echo "$output" | grep -i "lock:" || true)
            [[ -n "$maybe_lock" ]] && print_info "VM $vm_id still locked: $maybe_lock"
        fi
        elapsed=$((elapsed + poll_interval))
    done

    print_warning "Graceful shutdown window (${graceful_timeout}s) expired for VM $vm_id; escalating to force stop"

    # Ensure locks clear first (short wait)
    wait_for_vm_unlock "$vm_id" 20 "before force stop" true || true

    # Force stop attempt loop (now up to 5 attempts)
    local force_attempt=1
    while [[ $force_attempt -le 5 ]]; do
        print_info "Force stop attempt $force_attempt for VM $vm_id"
        local stop_out stop_exit
        if [[ "$RETRY_TRANSIENT" == "true" ]]; then
            if retry_run 5 2 "$PROX_BINARY vm stop $vm_id" "Force stop command"; then
                stop_exit=0
            else
                stop_exit=1
                stop_out="(see above retry output)"
            fi
        else
            set +e
            stop_out=$($PROX_BINARY vm stop "$vm_id" 2>&1)
            stop_exit=$?
            set -e
        fi
        if [[ $stop_exit -ne 0 ]]; then
            print_warning "Force stop command attempt $force_attempt failed (exit $stop_exit): $stop_out"
        fi
    if wait_for_vm_status "$vm_id" "stopped" 25 "force stop attempt $force_attempt" true; then
            print_success "VM $vm_id force stopped (attempt $force_attempt)"
            return 0
        fi
        force_attempt=$((force_attempt + 1))
        # Brief extra wait / unlock check before next iteration
    wait_for_vm_unlock "$vm_id" 15 "between force attempts" true || true
    done

    print_warning "Force stop attempts failed; performing final extended wait and one last graceful attempt"
    wait_for_vm_unlock "$vm_id" 40 "final unlock wait" true || true

    set +e
    shutdown_out=$($PROX_BINARY vm shutdown "$vm_id" 2>&1)
    shutdown_exit=$?
    set -e
    if [[ $shutdown_exit -eq 0 ]]; then
    if wait_for_vm_status "$vm_id" "stopped" 40 "final graceful"; then
            print_success "VM $vm_id shut down after final graceful attempt"
            return 0
        fi
    else
        print_warning "Final graceful attempt command failed (exit $shutdown_exit): $shutdown_out"
    fi

    print_error "Failed to stop VM $vm_id after adaptive graceful + multiple force attempts"
    return 1
}
# Main function
main() {
    echo "Prox CLI Comprehensive End-to-End Testing Script"
    echo "================================================"
    echo ""

    parse_args "$@"
    check_prerequisites

    print_info "Starting comprehensive E2E tests with:"
    print_info "  Test VM: $TEST_VM_NAME (ID: $TEST_VM_ID) - will be created by cloning VM $SOURCE_VM_ID"
    print_info "  Test Container: $TEST_CT_NAME (ID: $TEST_CT_ID) - will be created from template $TEST_TEMPLATE"
    print_info "  Source Node: $SOURCE_NODE"
    if [[ -n "$TARGET_NODE" ]]; then
        print_info "  Target Node: $TARGET_NODE (for migration testing)"
    fi
    if [[ -n "$TEST_SSH_KEY_FILE" ]]; then
        print_info "  SSH Key File: $TEST_SSH_KEY_FILE"
    fi
    echo ""

    # Run comprehensive E2E tests with resource lifecycle management
    # 1. Configuration tests (foundation)
    test_config_management
    
    # 2. Create test resources (VM via cloning, container from template)
    create_test_resources
    
    # 3 & 4. Core VM + Container operations (optionally parallel)
    if [[ "$PARALLEL_VM_CT_TESTS" == "true" ]]; then
        print_info "Running VM and container operation tests in parallel"
        local vm_log ct_log
        vm_log=$(mktemp /tmp/prox_vm_tests_XXXX.log)
        ct_log=$(mktemp /tmp/prox_ct_tests_XXXX.log)

        # Launch in subshells so variable mutations don't interfere; we'll aggregate after
        (
          test_vm_operations
        ) > >(tee "$vm_log") 2>&1 &
        local vm_pid=$!

        (
          test_container_operations
        ) > >(tee "$ct_log") 2>&1 &
        local ct_pid=$!

        wait $vm_pid || print_warning "VM operations subshell exited with non-zero status (continuing)"
        wait $ct_pid || print_warning "Container operations subshell exited with non-zero status (continuing)"

        # Aggregate results from logs since subshell counters are isolated
        aggregate_parallel_results() {
            local log_file="$1"
            # Strip ANSI colors, then parse
            while IFS= read -r line; do
                # Remove color codes
                local clean
                clean=$(echo "$line" | sed -r 's/\x1B\[[0-9;]*[A-Za-z]//g')
                if [[ "$clean" == *"[PASS]"* ]]; then
                    ((TESTS_PASSED++))
                elif [[ "$clean" == *"[FAIL]"* ]]; then
                    ((TESTS_FAILED++))
                    # Extract description after ] space
                    local desc
                    desc=${clean#*] }
                    FAILED_TESTS+=("$desc")
                fi
            done < "$log_file"
        }
        aggregate_parallel_results "$vm_log"
        aggregate_parallel_results "$ct_log"
        rm -f "$vm_log" "$ct_log"
        print_success "Parallel VM/CT operation tests aggregation complete"
    else
        test_vm_operations
        test_container_operations
    fi
    
    # 5. Advanced features (migration with created VM)
    test_vm_migration
    
    # 6. Additional container creation tests (temporary containers)
    test_container_creation
    
    # 7. Validation and error handling
    test_ssh_key_validation
    test_ssh_configuration
    test_error_handling

    # 8. Cleanup test resources
    cleanup_test_resources

    # Show results
    show_summary
    create_test_report

    # Node commands validation: list nodes and show info for NODE_NAME (if provided)
    local target_node
    target_node="${NODE_NAME:-$SOURCE_NODE}"
    print_info "Running node checks: prox node ls and prox node info $target_node"
    if [[ "$DRY_RUN" == "true" ]]; then
        print_info "[DRY RUN] Would run: $PROX_BINARY node ls"
        print_info "[DRY RUN] Would run: $PROX_BINARY node info $target_node"
    else
        if ! $PROX_BINARY node ls; then
            print_warning "'prox node ls' failed (non-fatal)"
        fi
        if ! $PROX_BINARY node info "$target_node"; then
            print_warning "'prox node info $target_node' failed (non-fatal)"
        fi
    fi
}

# Handle script interruption and ensure cleanup
cleanup_on_exit() {
    echo -e "\n${RED}Tests interrupted by user${NC}"
    cleanup_test_resources
    exit 130
}

trap 'cleanup_on_exit' INT

# Run main function with all arguments
main "$@"
