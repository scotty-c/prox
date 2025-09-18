# Architecture Overview

This document explains how Prox is structured so you can navigate, extend, and test it quickly.

## High-level Design

Prox is a thin CLI layer (Cobra) over a modular domain layer that wraps the Proxmox VE API. Responsibilities are deliberately separated:

| Layer | Responsibility |
|-------|----------------|
| `cmd/` | CLI command wiring, flags, argument parsing, human UX formatting orchestration only |
| `pkg/client/` | HTTP + auth + session handling + low-level endpoint helpers |
| `pkg/vm/` | VM-centric operations (lifecycle, clone, migrate, describe, list, edit) + task wait helpers |
| `pkg/container/` | LXC lifecycle + creation + template resolution + SSH key validation |
| `pkg/config/` | Secure config persistence (encryption, migration) |
| `pkg/conversion/` | Format & unit conversions for display |
| `tests/` | Unit + E2E harness (bash driven) |

Guiding principles:
- Single responsibility per file/package
- Shallow, composable functions (prefer small orchestrators over deep inheritance trees)
- Deterministic output & explicit errors
- Human friendly CLI output (tables) separated from core logic

## Data Flow (Typical Command)
1. Cobra command receives positional args & flags.
2. Command constructs / loads config (lazy decrypt).
3. Client authenticates or reuses ticket.
4. Domain package (vm/container) performs operation (start, list, etc.).
5. Domain returns structured data (structs) NOT formatted strings.
6. Command layer formats for terminal (tables, lines) and exits with appropriate code.

## Packages

### `pkg/client`
Responsibilities:
- Build & issue HTTP requests (GET/POST/DELETE) to Proxmox API
- Session ticket / CSRF management
- Error normalization (convert HTTP / API errors into Go `error`)
- Thin typed helpers around common endpoints

Key concepts:
- Minimal caching (stateless for predictability)
- Central place to update API schema changes

### `pkg/vm`
Responsibilities:
- VM lifecycle (start, shutdown, delete)
- Clone (full vs linked)
- Migration (online/offline, local disk flag) with task status polling
- Describe (rich aggregation: config, resources, IP detection)
- Edit (CPU, memory, rename, etc.)
- Listing (table-friendly slice)

Implementation notes:
- Uses Proxmox task IDs: `waitForTask` polls until completion or timeout
- IP resolution tries guest agent, then network config fallbacks
- Separation between fetch stage and render stage to enable reuse by tests or future APIs (JSON output possibility)

### `pkg/container`
Responsibilities:
- Container lifecycle: start, stop
- Create (template lookup, resource params, SSH keys injection)
- Describe & list operations
- Template shortcuts (e.g. `ubuntu:22.04`) mapped to actual storage paths
- SSH key validation (length, format, supported types) before submission

### `pkg/config`
Responsibilities:
- Persist credentials & URL to `~/.prox/config`
- Encrypt/decrypt sensitive fields (AES-256-GCM)
- Detect & migrate legacy plaintext
- Expose simple load/save API returning domain struct

### `pkg/conversion`
Utility helpers for:
- Human size formatting (MiB, GiB)
- Duration / uptime formatting
- Percentage & resource usage conversions

### CLI (`cmd/`)
Per resource group subfolder holds cohesive commands:
- `vm` commands attach verbs (list, start, clone, migrate ...)
- `ct` (containers) commands for list, templates, create, describe, start/stop
- `config` commands for setup/read/update/delete/migrate

Design rules:
- Keep logic minimal; delegate to packages
- Only assemble arguments, call domain, format output

## Error Handling Strategy
- Fail fast: return errors upward; CLI prints concise message
- Wrap context (e.g., VM ID, node) in errors for clarity
- User-level messages avoid leaking raw HTTP internals unless helpful
- Exit codes: non-zero on operational failure; 0 when expected errors are part of tests (handled in E2E harness separately)

## Security Model
- Encryption key derivation ties config file to host/user
- Only credentials (username/password) encrypted; URL stays plaintext
- File perms enforced (600)
- See `docs/SECURITY.md` for details

## Migration Flow (VM)
Short path for `prox vm migrate <id> <target>`:
1. Auto-discover source node (if omitted)
2. Validate target (and local disk flag if provided)
3. Fire migration task (online/offline)
4. Poll task -> progress -> final status
5. Return success / error

## Extensibility Guidelines
When adding a new feature:
1. Add core logic in domain package (return structs)
2. Add Cobra command wiring
3. Provide tests (unit or E2E) exercising success + failure path
4. Update docs if user-facing

Avoid:
- Embedding formatting inside logic packages
- Hidden global state
- Exporting unnecessary symbols

## Testing Layers
- Unit: fast validation of parsing, key validation, conversions
- E2E: real cluster operations (parallelizable sections, resource cleanup, leftover detection)
- Future: potential JSON output mode for machine verification

## Future Improvements (Ideas)
- JSON output flag for automation
- Parallel multi-VM operations (batch start/stop)
- Structured logging (debug JSON mode)
- Pluggable credential providers (env vault / secret store)

---
This should give you enough context to confidently navigate & extend Prox.
