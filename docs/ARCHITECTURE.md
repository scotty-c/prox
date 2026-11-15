# Architecture Overview

This document explains how Prox is structured so you can navigate, extend, and test it quickly.

## High-level Design

Prox is a Cobra CLI over packages that wrap the Proxmox VE API. A command parses its flags and calls a package; the package does the work and prints the result; any error is returned to `main`, which reports it once.

| Package | Responsibility |
|---------|----------------|
| `cmd/` (`cmd/vm`, `cmd/ct`, `cmd/config`, `cmd/node`) | Command wiring: flags, arguments, completion hooks. Commands are `RunE` and return errors; they do not print errors or call `os.Exit`. |
| `pkg/client` | HTTP, authentication (ticket or API token), TLS, typed API calls, task polling |
| `pkg/apierr` | Error kinds (`ErrUnreachable`, `ErrAuth`, `ErrPermission`, `ErrCertificate`, `ErrNoConfig`, `ErrNotFound`, `ErrAmbiguous`) used with `errors.Is` |
| `pkg/lookup` | Finding a VM or container by name or ID (shared by both), and the "is this ID free?" check |
| `pkg/vm`, `pkg/container` | Operations on VMs and containers: lifecycle, clone, migrate, edit, describe, list, create, templates. They print their own tables and result lines through `pkg/output`. |
| `pkg/snapshot` | Snapshot commands for both VMs and containers (one implementation, parameterised by guest type) |
| `pkg/node` | Node listing details |
| `pkg/cmdutil` | Helpers shared by commands: wait flags, `FinishTask`, confirmation prompts, bulk (`--tag`) operations, running `ssh` |
| `pkg/completion` | Shell completion (names, IDs, nodes, profiles) |
| `pkg/config`, `pkg/secrets`, `pkg/crypto` | Profiles under `~/.prox/profiles/`; `secrets` keeps the login in the OS keychain or seals it with a key file (and still reads the older derived-key format); `crypto` has the AES-GCM primitives |
| `pkg/output` | Everything that writes to the terminal: results, progress, errors with tips, spinners, `-o json/yaml` |
| `pkg/sshconfig` | Editing `~/.ssh/config` without disturbing the rest of the file |
| `pkg/util` | Small shared helpers (size and uptime formatting, fuzzy matching, tags, a bounded worker pool) |
| `internal/fakepve` | A fake Proxmox API for tests |
| `tests/cli`, `tests/e2e` | Tests that run the real binary (against the fake, and against a real cluster) |

Guiding principles:
- Single responsibility per file/package
- Shallow, composable functions
- One implementation for VMs and containers where Proxmox treats them alike (`lookup`, `snapshot`, `cmdutil`)
- Errors are values: returned, wrapped, classified with `errors.Is`, and printed once

## Data Flow (Typical Command)
1. `cmd.Execute` creates a context that Ctrl-C cancels and runs the Cobra command.
2. `PersistentPreRunE` applies the global flags (`--profile`, `--output`, `--insecure`, `--timeout`, `--quiet`).
3. The command's `RunE` calls a package function with `cmd.Context()`.
4. The package asks `client.CreateClient()` for the run's client. The first call reads the configuration (environment variables, else the current profile) and builds the client; later calls return the same one. The client logs in lazily on its first request and logs in again once if the ticket has expired.
5. The package finds the target with `lookup`, calls the API, waits for the task if asked (`cmdutil.FinishTask`), and prints the result.
6. A failure is returned up. `Execute` prints it once, with a tip chosen from the error's kind, and exits 1 (130 after Ctrl-C).

## Output Rules
- **stdout is the result** (a table, a line such as `VM web (ID 100) started in 1.2s`, or JSON/YAML). **stderr is everything else**: progress (`output.Info`), warnings, errors and tips. Piping a command therefore gives only its result.
- `--quiet` silences progress. `-o json` and `-o yaml` make list/describe commands print structured data, and commands that start a Proxmox task print one `TaskResult` object (see the README). Machine-friendly fields are added next to the human-formatted ones, never in place of them.
- Tables are rendered in the packages that own the data, with `go-pretty`.

## Errors
- Fail fast: return errors upward, wrapping them with what was being done (`fmt.Errorf("start failed: %w", err)`).
- The client classifies failures with the kinds in `pkg/apierr` (an unreachable server is not an authentication failure). `output.Fail` picks the tip from the kind, never by searching the message text.
- Lookup failures say whether a name was not found or is ambiguous, and include suggestions.
- Prompts that cannot be answered (stdin closed) are errors that name the flag to pass (`--yes`), not a silent "no".

## Context and Cancellation
`cmd.Context()` reaches every API request, so Ctrl-C stops whatever is in flight. Waiting for a task has its own message: the task keeps running on the server and the error shows the `prox logs --follow` command. `--timeout` bounds only the wait.

## The Client (`pkg/client`)
- Password logins use a ticket and CSRF token; usernames containing `!` are API tokens and send an `Authorization` header instead. For a profile in the keychain the last ticket is kept in the same keychain item (`secrets.Session`) and used while it is under ~110 minutes old; `newClient` applies it and passes each fresh login to `connection.SaveSession`. A rejected ticket still costs one re-login.
- TLS is verified unless `--insecure`/`PROX_INSECURE`/the profile says otherwise; a CA file can be trusted with `--ca-cert`.
- Responses are decoded straight into types (`requestData[T]`). A missing task ID or config is an error, not an empty value. Some values arrive as strings or numbers depending on the Proxmox version, so `ConfigNumber`/`ConfigFlag` accept both.
- The cluster listing (needed by every name lookup) is fetched once and kept for ten seconds; `ClearClusterResourcesCache` forgets it after a command creates a guest.
- "Unknown" is the empty string (an IP address that cannot be found, for example). The packages that display it decide how to show it (`N/A`).

## Configuration and Security
- Profiles live in `~/.prox/profiles/<name>`; `current-profile` names the active one. The login is kept by `pkg/secrets`: in the OS keychain (`credentials=keychain` in the profile, no secret in the file) or, with no keychain, sealed with a random key in `<user config dir>/prox/key` (`enc:v2:` values). Profiles from older versions (key derived from user and host) are read and upgraded by `prox config migrate`. The URL and TLS settings are plain text. Files are written 0600 through a temporary file and a rename.
- Environment variables (`PROX_URL`, `PROX_USER`, `PROX_PASSWORD`) take precedence over a profile when all three are set.
- See `docs/SECURITY.md` for the threat model.

## Extensibility Guidelines
When adding a command:
1. Put the logic in the package that owns the resource, taking a `context.Context` first.
2. Find targets with `lookup`, end task commands with `cmdutil.FinishTask`, and return errors instead of printing them.
3. Wire the Cobra command as `RunE`, with completion and `--wait`/`--no-wait` or `--yes` flags as appropriate (`cmdutil.AddWaitFlags`, `AddYesFlag`).
4. Add a test in `tests/cli` that runs the command against `internal/fakepve` and checks output, exit code and the requests made. Extend the fake if the endpoint is missing.
5. Update the README if it is user-facing.

Avoid:
- Printing errors from a package, or calling `os.Exit` outside `cmd.Execute` and the `ssh` exit-code passthrough
- Writing progress to stdout
- Package-level mutable state (the few process-wide settings set once from flags are the exception)
- Exporting symbols nothing outside the package uses

## Testing Layers
- **Unit** (`go test ./pkg/...`): parsing, formatting, lookup, client decoding against `httptest`.
- **CLI** (`tests/cli`): the real binary against `internal/fakepve`; see `docs/TESTING.md`.
- **E2E** (`tests/e2e`): real cluster operations, driven by a bash harness.
