# Prox

Simple CLI for Proxmox VE. Focus on: zero required --node flags (auto discovery), safe VM/CT lifecycle, secure encrypted config, fast migration & clear describe output.

## Contents
- [Why Prox?](#why-prox)
- [Install](#install)
- [Shell Completion](#shell-completion)
- [Quick Start](#quick-start)
- [Core Commands](#core-commands)
- [Flags Cheat Sheet](#flags-cheat-sheet)
- [Security](#security)
- [Advanced Docs](#advanced-docs)
- [Contributing](#contributing)
- [Releases](#releases)
- [License](#license)
- [Support](#support)

## Why Prox?
Key capabilities:
- Auto node discovery (omit --node 99% of the time)
- VM: list | start | shutdown | clone (full/linked) | migrate (online/offline, local disks) | edit | describe | delete
- Container (LXC): create (template shortcuts + SSH key injection), list, start/stop, describe
- Rich describe output: IP discovery (guest agent / DHCP / static), disks, resources, uptime
- Secure encrypted credential storage (AES‑256, automatic legacy migration)
- Multiple configuration profiles (homelab, production, etc.) with easy switching
- SSH key validation (file or stdin, multi‑key, type checks)
- Clear progress + helpful errors
- Modular Go codebase (easy to extend) + Makefile automation

## Install
Prereqs: Go 1.21.6+, access to Proxmox VE.
```bash
git clone https://github.com/scotty-c/prox.git
cd prox
make build        # outputs ./bin/prox
./bin/prox --help
```
Alt (quick):
```bash
go install github.com/scotty-c/prox@latest
prox --help
```
Docker / compose usage: see DOCKER.md.

## Shell Completion
Enable tab completion for your shell (bash, zsh, fish, or powershell):

**Bash:**
```bash
# Linux:
prox completion bash | sudo tee /etc/bash_completion.d/prox

# macOS:
prox completion bash > /usr/local/etc/bash_completion.d/prox
```

**Zsh:**
```bash
# Add to ~/.zshrc:
source <(prox completion zsh)

# Or generate to completion directory:
prox completion zsh > "${fpath[1]}/_prox"
```

**Fish:**
```bash
prox completion fish | source

# Or persist it:
prox completion fish > ~/.config/fish/completions/prox.fish
```

**PowerShell:**
```powershell
prox completion powershell | Out-String | Invoke-Expression

# Or add to your profile:
prox completion powershell >> $PROFILE
```

After setup, restart your shell or source the completion file. You'll then have tab completion for all commands, flags, and arguments.

## Quick Start
Configure (stored encrypted with profile support):
```bash
# Set up the default profile
prox config setup -u admin@pam -p secret -l https://proxmox.example.com:8006

# Or create multiple profiles for different environments
prox config create homelab -u admin@pam -p secret -l https://homelab:8006
prox config create production -u admin@pam -p secret -l https://prod:8006 --use
prox config list                    # List all profiles
prox config use homelab             # Switch to homelab profile
prox --profile production vm list   # Use specific profile for one command
```
VM workflow:
```bash
prox vm list
prox vm start 100
prox vm describe 100          # shows IPs, disks, resources
prox ssh 100                  # Add/update SSH config entry for VM 100
prox ssh --list               # List managed SSH config entries
prox ssh 100 --delete         # Remove SSH entry for VM 100
prox vm clone 100 101 clone-101 --full
prox vm migrate 100 node2 --online
prox vm shutdown 100
prox vm delete 101
```
Container workflow:
```bash
prox ct templates
# create: prox ct create <name> <template>
prox ct create web ubuntu:22.04 --ssh-keys-file ~/.ssh/id_rsa.pub --memory 1024 --disk 10
prox ct start web
prox ct describe web
prox ssh web          # Add/update SSH config entry for container
prox ct stop web
```
Advanced examples:
```bash
# Multiple SSH keys via stdin
cat ~/.ssh/id_*.pub | prox ct create --name build --template alpine:3.18 --ssh-keys-file -
# Edit VM resources
prox vm edit 100 --cpu 4 --memory 8192
# Complex migration
prox vm migrate 100 node2 --online --with-local-disks
# SSH setup with custom configuration
prox ssh web-server --user admin --port 2222 --key ~/.ssh/production_key
# Preview SSH entry creation/update
prox ssh web-server --dry-run
# Preview deletion (no file change)
prox ssh web-server --delete --dry-run
```
Pro tips:
- Skip --node: auto discovery handles it
- Aliases: describe→desc/show, delete→del/rm, shutdown→stop
- Template shortcuts: ubuntu:22.04 | debian:12 | alpine:3.18
- SSH key file can include multiple lines (each a key)
- Use `prox ssh <resource> --dry-run` to preview SSH config changes
- Use `prox ssh --list` to view current managed entries
- Use `prox ssh <resource> --delete [--dry-run]` to remove an entry safely

## Core Commands
Config (with profile support):
```bash
prox config setup -u user -p pass -l https://host:8006  # Set up default profile
prox config create <profile> -u user -p pass -l https://host:8006  # Create a new profile
prox config list                                         # List all profiles
prox config use <profile>                                # Switch to a different profile
prox config read                                         # Read current profile (masked)
prox config update                                       # Update current profile
prox config delete                                       # Delete current profile
prox --profile <name> [command]                          # Use specific profile for one command
```
VMs:
```bash
prox vm list | describe <id> | start <id> | shutdown <id> | delete <id>
prox vm clone <srcID> <newID> [NAME] [--full]
prox vm edit <id> --cpu 4 --memory 8192
prox vm migrate <id> <targetNode> [--online] [--with-local-disks]
```
Containers:
```bash
prox ct list | templates
# create: prox ct create <name> <template>
prox ct create api debian:12 --ssh-keys-file ~/.ssh/id_rsa.pub --memory 2048
prox ct start api; prox ct describe api; prox ct stop api
```
SSH Configuration:
```bash
# Add or update entry (VM or CT by name or ID)
prox ssh <vm-or-container-name-or-id>
# Custom options
prox ssh myvm --user root --port 22 --key ~/.ssh/id_rsa
# Dry-run (preview block, no write)
prox ssh 123 --dry-run
# List managed entries (table output similar to vm list)
prox ssh --list
# Delete an entry (by resource alias / name / ID)
prox ssh myvm --delete
# Dry-run delete
prox ssh myvm --delete --dry-run
```
Behaviour notes:
- Add/update: Rewrites (replaces) existing Host block matching the alias
- Delete: Safely removes only the targeted Host block; rest of config preserved
- List: Parses ~/.ssh/config and shows columns HOST | HOSTNAME | USER | PORT | IDENTITY
- Dry-run: Never modifies file (creation/update or deletion)

## Flags Cheat Sheet (human-friendly)

Containers (ct)
- create: prox ct create <name> <template>
	- Positional args:
		- name: container name (e.g., web, api)
		- template: either shortcut (ubuntu:22.04, debian:12, alpine:3.18) or full volid (storage:vztmpl/...) 
	- Flags:
		- -N, --name <name>          Alias for positional name (optional)
		- -t, --template <template>  Alias for positional template (optional)
		- -n, --node <node>            Create on a specific node (auto-resolves from template if omitted)
		-     --vmid <id>              Explicit container ID (auto-generated if omitted)
		- -m, --memory <MB>            Memory in MB (default 512)
		- -d, --disk <GB>              Disk size in GB (default 8)
		- -c, --cores <count>          CPU cores (default 1)
		- -p, --password <pwd>         Root password (use with care)
		-     --prompt-password        Prompt interactively for root password
		-     --ssh-keys-file <path|-> Public SSH key(s) file, or '-' to read from stdin (multi-line supported)
	- Notes:
		- Prefer positional name/template. If both positional and flags are supplied and conflict, positional wins; a warning is printed.
	- Examples:
		- prox ct create web ubuntu:22.04 --ssh-keys-file ~/.ssh/id_rsa.pub --memory 1024 --disk 10
		- cat ~/.ssh/id_*.pub | prox ct create build alpine:3.18 --ssh-keys-file -
		- prox ct create ci debian:12 --vmid 9002 --cores 2

- list
	- Flags: -n, --node <node>; -r, --running (only running)
	- Example: prox ct list --running

- describe <name|id>
	- Flags: none

- start <name|id> | stop <name|id> | delete <name|id>
	- Flags: none

- templates
	- Flags: -n, --node <node>

- shortcuts
	- Flags: none (prints common shortcuts like ubuntu:22.04)

VMs (vm)
- list
	- Flags: -n, --node <node>; -r, --running; -i, --ip (show IPs); -d, --detailed (disk info)

- describe <id|name>
	- Flags: -n, --node <node> (optional)

- start <id> | shutdown <id>
	- Flags: -n, --node <node> (optional)

- edit <id>
	- Flags:
		- -n, --node <node>
		- -N, --name <new-name>
		- -c, --cpu <cores>
		- -m, --memory <MB>
		- -d, --disk <GB>

- clone <sourceID> <newID> [NAME]
	- Flags: -n, --node <node>; -N, --name <name> (optional); -f, --full (full clone)
	- Notes: If NAME is provided positionally and --name is also set and they differ, positional NAME takes precedence; a warning is printed.

Nodes (node)
- list
	- Command: `prox node ls`
	- Description: List cluster nodes in a compact table (NAME, STATUS, TYPE, ID). Matches the VM/CT table UX.
	- Flags: - none currently (node auto-discovery is used when relevant)
	- Example:
		prox node ls

- describe <name|id> (alias: info)
	- Command: `prox node describe <name|id>` or `prox node info <name|id>`
	- Description: Show detailed information for a node: basic info, resource summary (CPU, memory, disk, uptime) and primary node IP if available.
	- Notes: Memory and disk are shown in GiB with percentages. Uptime is formatted as days/hours/minutes.
	- Example:
		prox node describe promox01

Nodes (node)
- list
	- Command: `prox node ls`
	- Description: List cluster nodes in a compact table (NAME, STATUS, TYPE, ID). Matches the VM/CT table UX.
	- Example:
		prox node ls

- describe <name|id> (alias: info)
	- Command: `prox node describe <name|id>` or `prox node info <name|id>`
	- Description: Show detailed information for a node: basic info, resource summary (CPU, memory, disk, uptime) and primary node IP if available.
	- Notes: Memory and disk are shown in GiB with percentages. Uptime is formatted as days/hours/minutes.
	- Example:
		prox node describe promox01

- Environment variable:
	- `PROX_NODE` can be set to specify a default node name for scripts/tests (see `tests/e2e/config.env`).

- migrate <id> <targetNode>
	- Flags: -s, --source <node>; --online; --with-local-disks

- delete <id>
	- Flags: -n, --node <node> (optional)

SSH (manage ~/.ssh/config entries)
- prox ssh <vm-or-ct name|id>
	- Flags: -u, --user <name>; -p, --port <num>; -k, --key <path>; --dry-run; --list; --delete
	- Notes: --list and --delete are mutually exclusive with add/update mode

Global config
- prox config setup -u/--username -p/--password -l/--url

Tip: run any command with --help for full details.

## Security
- AES‑256 encryption for stored credentials (system/user bound)
- Plaintext legacy configs auto‑migrated to profile system
- Profiles stored in `~/.prox/profiles/` with secure file perms (600)
- Automatic migration from old single-config format to profiles
- See SECURITY.md for encryption details & migration command
```bash
prox config migrate   # migrate legacy plaintext if present
```

## Advanced Docs
- [Architecture & package layout](docs/ARCHITECTURE.md)
- [Docker & container usage](docs/DOCKER.md)
- [Testing (E2E harness, categories)](docs/TESTING.md)
- [Security deep dive](docs/SECURITY.md)
- [Contributing workflow](CONTRIBUTING.md)

## Contributing
PRs welcome. See CONTRIBUTING.md (branch from main, add tests, run `make release-check`).

## Releases
GitHub Actions builds & publishes multi‑arch binaries + Docker images on version tags (`vX.Y.Z`). Check the Releases page.

## License
Apache 2.0 (see LICENSE).

## Support
Open an issue / discussion. Ensure your Proxmox account has rights for VM + CT operations. Include CLI version (`prox --version`) and command output when reporting problems.

---
Enjoy fast, safe Proxmox management with Prox.
