#!/usr/bin/env bash

# Prox E2E Test Setup Helper
# This script helps set up the environment for E2E testing

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="$SCRIPT_DIR/config.env"
EXAMPLE_CONFIG="$SCRIPT_DIR/config.env.example"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

print_info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to create config from example
create_config() {
    if [[ -f "$CONFIG_FILE" ]]; then
        read -p "Config file already exists. Overwrite? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Keeping existing config file"
            return
        fi
    fi

    cp "$EXAMPLE_CONFIG" "$CONFIG_FILE"
    print_success "Created config file: $CONFIG_FILE"
    print_info "Please edit $CONFIG_FILE with your environment details"
}

# Function to validate config
validate_config() {
    if [[ ! -f "$CONFIG_FILE" ]]; then
        print_error "Config file not found: $CONFIG_FILE"
        print_info "Run '$0 setup' to create one"
        return 1
    fi

    source "$CONFIG_FILE"

    local errors=0

    # Check if prox is configured first
    local binary_path="$SCRIPT_DIR/$PROX_BINARY"
    if [[ ! -f "$binary_path" ]]; then
        print_error "Prox binary not found at: $binary_path"
        print_info "Build the binary first: make build (from project root)"
        ((errors++))
    else
        # Check if prox is configured
        if ! "$binary_path" config read &>/dev/null; then
            print_error "Prox is not configured. Run 'prox config setup' first."
            print_info "The E2E tests use your existing prox configuration for Proxmox connection details."
            ((errors++))
        fi
    fi

    # Check required variables for comprehensive testing
    if [[ -z "$TEST_VM_NAME" || -z "$TEST_VM_ID" ]]; then
        print_error "TEST_VM_NAME and TEST_VM_ID must be set"
        ((errors++))
    fi

    if [[ -z "$TEST_CT_NAME" || -z "$TEST_CT_ID" ]]; then
        print_error "TEST_CT_NAME and TEST_CT_ID must be set"
        ((errors++))
    fi
    
    if [[ -z "$SOURCE_VM_ID" ]]; then
        print_error "SOURCE_VM_ID must be set (existing VM to clone for testing)"
        ((errors++))
    fi
    
    if [[ -z "$SOURCE_NODE" ]]; then
        print_error "SOURCE_NODE must be set (node where test resources will be created)"
        ((errors++))
    fi
    
    if [[ -z "$TEST_TEMPLATE" ]]; then
        print_error "TEST_TEMPLATE must be set (container template for testing)"
        ((errors++))
    fi

    # Check SSH key file if specified
    if [[ -n "$TEST_SSH_KEY_FILE" && ! -f "$TEST_SSH_KEY_FILE" ]]; then
        print_error "SSH key file not found: $TEST_SSH_KEY_FILE"
        ((errors++))
    fi

    if [[ $errors -eq 0 ]]; then
        print_success "Configuration validation passed"
        print_info "Using existing prox configuration for Proxmox connection"
        return 0
    else
        print_error "Configuration validation failed with $errors errors"
        return 1
    fi
}

# Function to discover available test resources
discover_resources() {
    source "$CONFIG_FILE" 2>/dev/null || true
    local binary_path="$SCRIPT_DIR/$PROX_BINARY"
    
    if [[ ! -f "$binary_path" ]]; then
        print_error "Prox binary not found at: $binary_path"
        print_info "Build the binary first: make build (from project root)"
        return 1
    fi

    if ! "$binary_path" config read &>/dev/null; then
        print_error "Prox is not configured. Run 'prox config setup' first."
        return 1
    fi

    print_info "Discovering available resources for comprehensive E2E testing..."
    echo ""
    
    print_info "Available VMs (for cloning as source):"
    if ! "$binary_path" vm list; then
        print_error "Failed to list VMs"
        return 1
    fi
    
    echo ""
    print_info "Available Container Templates:"
    if ! "$binary_path" ct templates | head -20; then
        print_error "Failed to list container templates"
        return 1
    fi
    
    echo ""
    print_info "Available Nodes:"
    if ! "$binary_path" vm list | grep -o "Node: [^,]*" | sort -u; then
        print_warning "Unable to extract node information from VM list"
    fi
    
    echo ""
    print_info "Configuration guidance for comprehensive E2E testing:"
    echo "=================================================="
    print_info "1. Choose a SOURCE_VM_ID from the VMs listed above (will be cloned, not modified)"
    print_info "2. Choose available TEST_VM_ID and TEST_CT_ID that don't exist yet"
    print_info "3. Set SOURCE_NODE to where you want test resources created"
    print_info "4. Optionally set TARGET_NODE for migration testing (different from SOURCE_NODE)"
    print_info "5. Choose TEST_TEMPLATE from the templates listed above"
    echo ""
    print_info "Example config.env setup:"
    echo "  SOURCE_VM_ID=\"100\"           # Existing VM to clone"
    echo "  TEST_VM_ID=\"9001\"            # Available ID for test VM"
    echo "  TEST_CT_ID=\"9002\"            # Available ID for test container"
    echo "  SOURCE_NODE=\"proxmox01\"      # Node for test resources"
    echo "  TARGET_NODE=\"proxmox02\"      # Different node for migration testing"
    echo "  TEST_TEMPLATE=\"ubuntu:22.04\" # Container template"
}

# Function to run tests with config
run_tests() {
    if ! validate_config; then
        exit 1
    fi

    source "$CONFIG_FILE"

    local args=(
        "--vm-name" "$TEST_VM_NAME"
        "--vm-id" "$TEST_VM_ID"
        "--ct-name" "$TEST_CT_NAME"
        "--ct-id" "$TEST_CT_ID"
        "--source-vm-id" "$SOURCE_VM_ID"
        "--source-node" "$SOURCE_NODE"
        "--template" "$TEST_TEMPLATE"
        "--prox-binary" "$PROX_BINARY"
    )

    if [[ -n "$TEST_SSH_KEY_FILE" ]]; then
        args+=("--ssh-key-file" "$TEST_SSH_KEY_FILE")
    fi

    if [[ -n "$TARGET_NODE" ]]; then
        args+=("--target-node" "$TARGET_NODE")
    fi

    if [[ -n "$TEMPLATE_NODE" ]]; then
        args+=("--template-node" "$TEMPLATE_NODE")
    fi

    if [[ "$VERBOSE" == "true" ]]; then
        args+=("--verbose")
    fi

    if [[ "$DRY_RUN" == "true" ]]; then
        args+=("--dry-run")
    fi

    if [[ "$SKIP_CLEANUP" == "true" ]]; then
        args+=("--skip-cleanup")
    fi

    print_info "Running E2E tests with configuration..."
    exec "$SCRIPT_DIR/run_e2e_tests.sh" "${args[@]}"
}

# Function to show current config
show_config() {
    if [[ ! -f "$CONFIG_FILE" ]]; then
        print_error "Config file not found: $CONFIG_FILE"
        return 1
    fi

    print_info "Current configuration:"
    echo "======================="
    
    source "$CONFIG_FILE"
    
    echo "Proxmox Connection:"
    echo "  Using existing prox configuration (run 'prox config read' to view)"
    echo ""
    echo "VM Details:"
    echo "  Name: $TEST_VM_NAME"
    echo "  ID: $TEST_VM_ID"
    echo ""
    echo "Container Details:"
    echo "  Name: $TEST_CT_NAME"
    echo "  ID: $TEST_CT_ID"
    echo ""
    echo "Optional Settings:"
    echo "  SSH Key File: ${TEST_SSH_KEY_FILE:-"Not set"}"
    echo "  Target Node: ${TARGET_NODE:-"Not set"}"
    echo "  Prox Binary: $PROX_BINARY"
    echo ""
    echo "Test Options:"
    echo "  Verbose: $VERBOSE"
    echo "  Dry Run: $DRY_RUN"
    echo "  Skip Cleanup: $SKIP_CLEANUP"
}

# Function to show usage
show_usage() {
    cat << EOF
Prox E2E Test Setup Helper

Usage: $0 <command>

Commands:
  setup       Create configuration file from example
  discover    List available VMs and containers to help configure tests
  validate    Validate current configuration
  run         Run E2E tests using configuration file
  config      Show current configuration
  help        Show this help message

Examples:
  $0 setup      # Create config.env from example
  $0 discover   # List available VMs/containers for configuration
  $0 validate   # Check if configuration is correct
  $0 run        # Run the E2E tests
  $0 validate   # Check if configuration is valid
  $0 run        # Run tests with current configuration
  $0 config     # Show current settings

Configuration:
  The script uses config.env for test parameters (VM/container details, SSH keys, etc.).
  Proxmox connection details are read from your existing prox configuration.
  If config.env doesn't exist, run 'setup' to create it from the example.

EOF
}

# Main function
main() {
    case "${1:-help}" in
        setup)
            create_config
            ;;
        discover)
            discover_resources
            ;;
        validate)
            validate_config
            ;;
        run)
            run_tests
            ;;
        config)
            show_config
            ;;
        help|--help|-h)
            show_usage
            ;;
        *)
            echo "Unknown command: $1"
            show_usage
            exit 1
            ;;
    esac
}

main "$@"
