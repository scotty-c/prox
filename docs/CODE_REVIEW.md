# prox: Code Review and Improvement Suggestions

**Date:** 2026-11-11
**Branch reviewed:** `updates` (HEAD `9c06898`)
**Scope:** All Go sources under `cmd/` and `pkg/`, build/CI files, and the Dockerfile. I read the e2e scripts only for context.

---

## How to read this

prox has users, so every suggestion below is rated by **compatibility risk**:

- **Safe**: internal change. CLI surface, flags, output and config files all stay the same.
- **Additive**: adds a new flag, command or env var. Existing behaviour is unchanged.
- **Behaviour change**: something users or scripts could notice. Each one comes with a migration path, such as a deprecation period, an opt-in flag, or a default that only applies in new cases.

Priorities:

- **P0**: a real bug that users hit today.
- **P1**: a correctness, security or reliability problem that is worth fixing soon.
- **P2**: quality, consistency or maintainability.
- **P3**: nice to have, or a new feature.

I confirmed the bugs marked **(verified)** with a small scratch program run against the exact logic in the repo. The others come from reading the code.

---

## Summary

The codebase is in reasonable shape. The command structure is clear, name/ID lookup with fuzzy suggestions is a nice touch, and the IP lookups use a sensible worker pool. The main themes are:

1. **A few functional bugs on common paths.** `ct create debian:12` fails, `--ssh-keys-file -` never reads any keys, and several commands exit 0 when they fail.
2. **Values that look like data but are made up.** Disk usage figures are estimated and returned as if they were real, including in `--json` output.
3. **Weak points in credential handling.** If the hostname changes, decryption silently returns garbage. There is no API token support, even though the help text says there is. TLS verification is always off.
4. **Inconsistency between commands.** Exit codes, stdout vs stderr, wait/no-wait defaults, and `--node` flags that are accepted but ignored.
5. **Almost no unit tests.** There is one test file. The `ProxmoxClientInterface` seam exists but nothing uses it for testing.

None of the fixes need a breaking change. Where behaviour does change, it moves toward what users already expect.

---

## P0: Bugs users hit today

### 1. `prox ct create <name> debian:12` cannot resolve the template (verified)
`pkg/container/utils.go:47` `parseTemplateVersion` splits the volume ID on `-` and returns the first part that contains a `.` and is at most 10 characters long.

| Template volid | Parsed version |
|---|---|
| `ubuntu-22.04-standard_22.04-1_amd64.tar.zst` | `22.04` OK |
| `alpine-3.20-default_20240908_amd64.tar.xz` | `3.20` OK |
| `debian-12-standard_12.7-1_amd64.tar.zst` | **`Unknown`** |
| `rockylinux-9-default_20240912_amd64.tar.xz` | **`Unknown`** |

Debian is the most common LXC base on Proxmox, and `debian:12` is listed as a supported shortcut in `prox ct shortcuts` and the README. Because the version comes back as `Unknown`, both the exact-match and contains-match lookups fail.

- **Fix:** parse the volid as `<os>-<version>-<flavour>_...`. Take the file name after `vztmpl/`, split on the first `-`, and use the next segment up to `-` as the version. That gives `debian`/`12`, `ubuntu`/`22.04` and `rockylinux`/`9`. Add a table-driven test with real volids.
- **Related:** `parseTemplateDescription` (`utils.go:25`) only strips `.tar.gz`, but current templates are `.tar.zst` or `.tar.xz`. `parseTemplateOS` returns `Unknown` for Rocky, Alma, Arch, openSUSE, Gentoo and others. Using the volid prefix as the OS fixes both.
- **Risk:** Safe. Inputs that resolved before still resolve to the same template.

### 2. `ct create --ssh-keys-file -` (stdin) never reads any keys (verified)
`cmd/ct/create.go:129` uses `fmt.Scanln(&line)`, which reads **one whitespace-delimited token**. With an SSH key line (`ssh-ed25519 AAAA… user@host`) it reads `ssh-ed25519` and then returns `expected newline`. The loop breaks before appending, so `sshKeys` stays empty and the user only sees "No valid SSH keys found", or nothing at all.

- **Fix:** read stdin with `io.ReadAll(os.Stdin)` or a `bufio.Scanner`. Only print the "press Ctrl+D" hint when stdin is a TTY (`term.IsTerminal`).
- **Risk:** Safe. The feature is currently broken.

### 3. Several commands exit 0 when they fail
This breaks `set -e` scripts and CI pipelines, which the README says are supported.

| Location | Problem |
|---|---|
| `pkg/vm/operations.go:281` `DeleteVM` | Has no return value. Every error is printed and swallowed, and `DeleteVMByNameOrID` always returns `nil` (`:459-460`). |
| `pkg/vm/config.go:35` `EditVM` | Same pattern. `EditVMByNameOrID` always returns `nil`. |
| `pkg/container/templates.go:15` `ListTemplates` | No error return. |
| `cmd/config/{setup,read,delete,create,use,list}.go` | `Run` prints `Error …` and returns, so the exit code is 0. |
| `pkg/vm/list.go` `ListVMs` | Returns `nil` on "No virtual machines found" but prints `Error:`. This one is arguably fine (an empty list is not an error), but the `Error:` prefix should go. |

- **Fix:** have these functions return `error`, and have the `cmd` layer call `os.Exit(1)`, or better, switch to `RunE` (see P2 #14).
- **Risk:** Behaviour change, but a correct one. A script that relied on `prox vm delete` "succeeding" when it failed was already broken. Mention it in the release notes.

### 4. `--node` is accepted but ignored on `vm start`, `shutdown`, `delete`, `edit` and `clone`
The flag is registered (`cmd/vm/start.go:38`, `shutDown.go:39`, `delete.go:50`, `edit.go:43`, `clone.go:70`) and documented in `--help`, but the `Run` functions never read it. A user who passes `--node pve2` to choose between two VMs with the same name gets the first match on any node and no warning.

- **Fix:** either pass `node` through to `FindByNameOrID` and filter on it, which is the expected behaviour, or mark the flag hidden and deprecated with `MarkDeprecated`. Don't remove it, because that would break scripts that pass it.
- **Risk:** Safe if wired through. Today the flag is a no-op, so any value a user passes now becomes a filter. If it doesn't match, the command should fail with a clear error rather than act on the wrong VM.

### 5. `prox status` ignores the `PROX_*` environment variables
`cmd/status.go:1023-1031` calls `config.Check()` and `config.Read()` directly. A CI user with only `PROX_URL`, `PROX_USER` and `PROX_PASSWORD` set gets "no configuration found", even though every other command works. The root help says env vars work "without config file".

- **Fix:** use `client.ReadConfig()` (which already handles env-then-file) and drop the `config.Check()` gate.
- **Risk:** Safe.

### 6. `prox ssh` gives a confusing error when there is no IP
`GetVMIP` and `GetContainerIP` return the sentinel `"N/A"`, not `""`. The `ip == ""` check at `cmd/ssh.go:139` never fires, so the user sees `invalid IP address: N/A` instead of the helpful "Make sure the resource is running and has an IP assigned".

- **Fix:** check for `"N/A"` here. Longer term, return `("", ErrNoIP)` from the client (see P2 #12).
- **Risk:** Safe.

### 7. `ct create --node X` is silently overridden
`pkg/container/create.go:29-40`: if the user asks for node `X` and the template was found on node `Y` first, prox prints a warning and **creates the container on `Y`**. With shared template storage (NFS, CephFS, PBS), the template exists on every node, so the user can never target a specific node.

- **Fix:** collect *all* nodes that hold the template. If the requested node is among them, use it. Only fail (rather than silently switch) when the template really isn't available there.
- **Risk:** Behaviour change, toward what the flag promises. Today's override is surprising and probably not relied on.

---

## P1: Correctness, security, reliability

### 8. Credential decryption is fragile and fails silently
`pkg/crypto/crypto.go`:

- **Key derivation uses the hostname** (`generateKey`). On macOS, `os.Hostname()` can change with network or DHCP (`MacBook-Pro.local` → `MacBook-Pro-2.local`), and laptops get renamed. When that happens…
- …`Decrypt` **returns the raw decoded bytes as the plaintext** when GCM authentication fails (`:119-123`). The user gets a mystery "authentication failed 401" against Proxmox with a garbage password, instead of "config was encrypted on a different machine/hostname, re-run `prox config setup`".
- **Plaintext detection is ambiguous** (verified). Any plaintext password made only of base64 characters with a length that is a multiple of 4 (`admin123`, `password`, `Passw0rd`) passes `isBase64`. Short ones then fail with `ciphertext too short`. Long ones get decoded into garbage. This only affects legacy plaintext configs, but it is exactly the migration path.
- The config file already records a key fingerprint (`config.go:190`), but nothing checks it.

**Fixes (all compatible with existing files):**

1. In `Decrypt`, return an error when GCM fails instead of falling through. Have `output.ClientError` map that error to a "re-run `prox config setup`" tip (the `decrypt` branch already exists).
2. When reading a profile, compare the stored fingerprint with the current one and emit a specific error if they differ.
3. Write new files with a version prefix (`enc:v1:<b64>`), so plaintext vs ciphertext is unambiguous from now on. Keep reading un-prefixed values the current way for backward compatibility.
4. Longer term (Additive): use the OS keychain via `zalando/go-keyring`, with the current file-based scheme as a fallback. Optionally derive the key from a random per-install secret stored in `~/.prox/key` (0600) instead of from the hostname. That makes it stable across renames, though it is still not secure against someone who can read your home directory, which is fine and should be documented.

The README says "Secure encrypted configuration storage with AES-256". `docs/SECURITY.md` should say plainly that this is obfuscation at rest: anyone who can run code as the user can derive the key. That isn't a code change, but it sets honest expectations.

### 9. No API token support, although the help text says "Password or API token"
`cmd/root.go:38` documents `PROX_PASSWORD` as "Password or API token", but `Authenticate` only does ticket auth. Proxmox API tokens need the header `Authorization: PVEAPIToken=user@realm!tokenid=<secret>` and no ticket call. API tokens are the recommended way to automate PVE: they are scoped, revocable, and work for accounts with 2FA.

- **Fix (Additive):** add `PROX_TOKEN_ID` and `PROX_TOKEN_SECRET`, or detect a `!` in the username, plus `--token-id` and `--token-secret` on `config setup` and `config create`. In `makeRequest`, set the `Authorization` header and skip `Authenticate` and the CSRF header when a token is configured.
- Until then, fix the help text so people stop trying it.

### 10. TLS verification is always off
`pkg/client/client.go:182-184` hardcodes `InsecureSkipVerify: true`. That is a pragmatic default for home labs with self-signed certificates, but users with proper certificates (ACME is built into PVE) have no way to turn verification on, and it makes credential interception trivial on hostile networks.

- **Fix (Additive):** add `--insecure` and `PROX_INSECURE`, and an optional `--ca-cert` / `PROX_CA_CERT`, stored per profile. **To avoid breaking existing users:** keep insecure as the default for existing profiles (no setting stored means insecure). For *new* profiles, `config setup` can try a verified connection first and fall back to insecure with a one-line warning. Optionally add certificate fingerprint pinning (`PROX_FINGERPRINT`), the way the Proxmox Backup client does.

### 11. `prox ssh` writes weak and possibly destructive SSH config
`cmd/ssh.go`:

- **`StrictHostKeyChecking no` and `UserKnownHostsFile /dev/null`** (`:224-225`) are written for every host. This turns off MITM protection permanently for those aliases. It is convenient for disposable VMs, but it should be opt-in (`--no-host-key-check`), or at least `StrictHostKeyChecking accept-new` (TOFU), which keeps the convenience. *Behaviour change:* existing entries are untouched. Only newly written entries change, so it is low risk.
- **It can delete unrelated config.** `addToSSHConfig` and `removeFromSSHConfig` skip lines until the next `Host` line. If the target block is followed by a `Match` block, or comments that belong to the next block, those get deleted too. Treat `Match` as a block boundary as well.
- **No backup, and the write isn't atomic.** Write to `config.tmp` and rename it over the original, keeping a single `config.prox.bak`. Watch for `~/.ssh/config` being a symlink, which is common with dotfile managers. Resolve it with `filepath.EvalSymlinks` first so the rename doesn't replace the symlink.
- **IPv4 only** (`isValidIP`). Use `net.ParseIP`, which also accepts IPv6. `extractIPFromInterfaces` also filters to IPv4 only, so IPv6-only guests show `N/A`.
- **The default SSH user is the VM name** (`:155`). It is unlikely that a VM called `web-server` has a user `web-server`. Consider defaulting to the current `$USER`, or omitting `User` entirely so `~/.ssh/config` defaults apply. *Behaviour change:* only affects newly written entries. Could also be gated behind a new flag.
- `listSSHConfigEntries` (`:459`) indexes into `trim[8:]` etc. That works because of the prefix checks, but it breaks on tab-separated or `=`-separated directives (`HostName=1.2.3.4` is valid ssh_config syntax). Use `strings.Fields` or split on `=`.

### 12. Disk usage numbers are made up and shown as real data
- `pkg/vm/list.go:296-303`: when `disk` is 0, which is **always** the case for QEMU VMs in `/cluster/resources` because PVE doesn't report guest disk usage there, prox sets `Disk = MaxDisk/5` (running) or `/10` (stopped). That goes straight into **`--json` output**.
- `pkg/vm/list.go:318-323`: sets `MaxDisk = 1` as a "has disks" sentinel. That also leaks into JSON as `"max_disk": 1`.
- `pkg/client/client.go:1527-1538`: `GetVMDiskInfo` makes the same 20%/10% estimate.
- `pkg/client/client.go:1656-1661`: `getVMDiskUsageFromRRD` returns `diskwrite rate × 3600` as "used bytes". That is an I/O rate, not a capacity, so the number is meaningless.

Anyone using `vm list --json` for capacity monitoring is getting invented numbers.

- **Fix:** report unknown values as unknown. In tables, show `N/A` or just the allocated size. In JSON, add `"disk": null` or omit it. The real source for used space is the guest agent (`agent/get-fsinfo`), which `--detailed` could use. Remove the RRD heuristic.
- **Risk:** Behaviour change for JSON consumers. The values were wrong, though, so this is a bug fix. To be careful, keep the `disk` key but emit `0` or `null`, and document it in the changelog.

### 13. `WaitForTask` on the client is a busy loop
`pkg/client/helpers.go:37-57`: the `select` has a `default:` branch and no sleep, so it hammers `GET /tasks/{upid}/status` as fast as the network allows. Nothing calls it today (both packages have their own `waitForTask`), but it is exported on the public interface and is a trap for the next contributor.

- **Fix:** delete it, or make it the single shared implementation, with backoff, used by both `pkg/vm` and `pkg/container` (they currently have identical copies in `vm/utils.go:115` and `container/utils.go:97`). Also delete the unused constant `TaskPollIntervalSeconds`.

### 14. Smaller correctness issues
- **`GetID` only checks `qemu` IDs** (`pkg/vm/operations.go:27`). Cloning onto an ID used by an **LXC container** passes the check and then fails at the API with a less friendly error. Check all resource types.
- **`MigrateVM` prints "VM is now running on node X"** (`:379`) even for offline migrations of stopped VMs. Say "is now on node".
- **CPU percentage is computed two ways** in `vm/utils.go`: `:47` uses the constant and `:82` hardcodes `100`. Extract a single `vmFromResource(Resource) VM` helper. The ID path and the name path duplicate about 30 lines.
- **`FindByNameOrID` silently takes the first match** when two VMs share a name (Proxmox allows that). Return an "ambiguous name, use the ID" error listing the matches. Behaviour change, but it prevents acting on the wrong VM. It matters most for `delete`.
- **`config.DeleteProfile`** (`pkg/config/config.go:211-217`): if you delete the active profile and `default` doesn't exist, `current-profile` still points at the deleted profile, and every later command fails with "profile does not exist". Remove `current-profile` in that case.
- **`config.CreateProfile`** creates the file with `os.Create` (default 0666 & umask) and then `Chmod`s it, which leaves a brief window. It also truncates in place, ignores `Fprintf` errors, and isn't atomic, so a failure partway through leaves a corrupt profile. Use `os.OpenFile(..., 0600)`, write to a temp file, then rename.
- **Profile names aren't validated.** `--profile ../../something` resolves outside `~/.prox/profiles`. This is low impact because it is the user's own account, but a `^[A-Za-z0-9._-]+$` check is cheap.
- **`config setup` on an existing config** prints "Config already exists" and then "Config setup completed successfully" (`cmd/config/setup.go`), which contradicts itself. Also, passing the password with `-p` puts it in shell history and `ps`. Prompt for any missing values when stdin is a TTY (Additive: the flags still work).
- **`config read` prints the password in clear text.** Mask it by default and add `--show-password`. This is a behaviour change, but low risk, since scripts shouldn't be scraping this output.
- **`ReadConfig` only uses env vars when all three are set** (`client.go:755`). If someone sets only `PROX_URL` to override one value, it is silently ignored. Either merge per field (env overrides profile) or warn when the env vars are only partly set. Per-field merge is Additive in practice.
- **The `vm edit --disk` help says "Disk size in GB"**, but the code *grows the disk by* that amount (`+%dG`). Users will expect `--disk 50` to mean "make it 50 GB". Fix the help text to "Grow disk by N GB". Don't change the semantics, because that would be breaking and dangerous.
- **Broken characters (`�`) in output** at `pkg/vm/config.go:108` and `:142`, left over from the emoji removal.
- **The `.gitignore` path has a typo:** `test/e2e/config.env` should be `tests/e2e/config.env`. Your local e2e config (node names) is currently tracked. There are no secrets in it today, but the next person who fills in a password will commit it.

---

## P2: Consistency, UX polish and maintainability

### 15. Standardise command plumbing on `RunE`
Today most commands use `Run` plus `os.Exit(1)` at many call sites, and a few use `RunE`. Because `SilenceUsage` isn't set, any runtime error from a `RunE` command (`ssh`, `status`) prints the **full usage text** after the error, which looks like the user typed something wrong.

- Set `RootCmd.SilenceUsage = true` and `SilenceErrors = true`, and print errors once in `Execute()` through `output.Error…`. Argument validation errors still show usage, because cobra validates args before `RunE` and you can call `cmd.Usage()` explicitly there.
- Convert `Run` to `RunE` gradually, one command group per PR.
- **Risk:** Safe. Output only changes by losing the spurious usage dump.

### 16. Write errors and progress to stderr, results to stdout
Many error paths use `fmt.Printf("Error: …")`, which goes to stdout (`cmd/vm/clone.go:35`, `cmd/logs.go:854`, `cmd/ct/create.go`, the whole `pkg/vm/config.go`, the `cmd/config/*` commands). The spinner (`briandowns/spinner`) also writes to stdout by default. This corrupts `--json` output and pipes.

- Route everything through `pkg/output`. Set `s.Writer = os.Stderr` on the spinner, and skip it when `QuietMode` is set or stdout isn't a TTY.
- `config.MigrateToProfiles` prints with `output.Resultln` (stdout) from inside `config.Read()`, so the first run after an upgrade emits migration text in the middle of `vm list --json`. Send it to stderr.

### 17. Make wait behaviour consistent
| Command | Waits by default? |
|---|---|
| `vm start` / `vm shutdown` | No (`--wait` to opt in) |
| `vm clone` | Yes (`--no-wait` to opt out) |
| `vm migrate` | Always, no flag |
| `vm delete` | Never, no flag |
| `ct start` / `stop` / `delete` / `create` | Always, no flag |

Don't change the defaults, since that would break UX. Instead, **add** the missing opt-in or opt-out flags so every command supports both `--wait` and `--no-wait`, and document the default in each command's help. Also consider `--timeout`: today a task that hangs blocks forever, because every call uses `context.Background()`. Using `signal.NotifyContext(ctx, os.Interrupt)` at the root gives clean Ctrl-C handling during waits, and lets the command print the UPID so the user can run `prox logs` afterwards.

### 18. Add `vm stop --force` (hard stop)
`vm stop` is an alias for **graceful shutdown** (ACPI). A VM without ACPI or the guest agent never stops, and there is no hard-stop path (`/status/stop`). Add `--force` on `shutdown`/`stop` (Additive), and do the same for `ct stop`. Also consider `vm reboot` and `vm reset`.

### 19. Reduce duplication in the client
`client.go` is 1,767 lines, and much of it repeats the pattern *request → unmarshal into `interface{}` → type-assert `Data.(string)`*. About 15 methods are the same 15 lines.

- Add one generic helper: `func doJSON[T any](c, ctx, method, path string, body any) (T, error)` that unmarshals into `struct{ Data T }`. Each method becomes 3 lines and gets real typed structs instead of `map[string]interface{}`.
- Remove dead code: `getVMIPFromClusterResources` (always returns `N/A`), `getStorageVolumeSize` (superseded by the cached version), `GetVM()` in `vm/list.go` (unused), `VirtualMachine.Config` (always errors), the unused `pkg/conversion` package, and the unused constants `DiskInterfaces` and `ContainerTemplateShortcuts`.
- `GetVMIP` tries five strategies. `getVMIPFromInterfaces` calls `/nodes/{n}/qemu/{id}/interfaces`, which **doesn't exist for QEMU** (it is an LXC endpoint), and `getVMIPFromStatus` looks for a `net` key that `status/current` doesn't return. That is two wasted round-trips for every VM without the guest agent, and it makes `vm list --ip` slow. Keep the guest agent and `ipconfigN` (cloud-init) only, and parse the `ipconfig0` key rather than `net0`, since `ip=` lives in `ipconfigN` and not `netN`.
- Return errors or `""` instead of the `"N/A"` sentinel from the client. Formatting belongs in the presentation layer.

### 20. HTTP client details
- Path segments aren't escaped. Node names and UPIDs are safe in practice, but `url.PathEscape` on each interpolated segment costs nothing.
- When a ticket expires mid-command (2 h), as can happen during `logs --follow` on a long task, every request fails. On a 401, re-authenticate once and retry.
- `Authenticate` isn't guarded by a mutex. Concurrent IP-lookup workers start after the first request has already authenticated, so it isn't a live race today, but a `sync.Once` or mutex makes it robust.
- Error messages embed the full raw response body. For HTML error pages that is noisy. Truncate it, or parse the `errors` and `message` fields from the PVE JSON.

### 21. Shell completion
- `GetProfileNames` is a stub (`pkg/completion/completion.go:96`) even though `config.ListProfiles()` exists. Wire it to `config use`, `config delete` and `--profile` with `RegisterFlagCompletionFunc`.
- Register `GetNodeNames` for every `--node`, `--source` and `vm migrate <target>` argument.
- Completion functions should return names only after the first argument (`len(args) > 0 → NoFileComp`), otherwise `prox vm clone a <TAB>` suggests VM names for the new ID.

### 22. `ct create` hardcodes
`net0` is hardcoded to `bridge=vmbr0,ip=dhcp`, and the default storage is `local-lvm` (which doesn't exist on ZFS installs, where it is `local-zfs`). Add `--bridge`, `--ip` and `--vlan` (Additive). For storage, if the user didn't pass `--storage` and `local-lvm` doesn't exist on the node, pick the first storage with `rootdir` content and say so. That keeps the current default where it works.

---

## P3: Testing, CI, release

### 23. Unit tests
There is one test file (`pkg/container/ssh_keys_test.go`). The highest-value targets need no Proxmox server:

- **Pure functions:** `parseTemplateVersion`, `parseTemplateOS`, `parseUPIDNode`, `parseDiskSize`, `ValidateDiskSize`, `extractIPFromInterfaces`, `hasTag`, `FindSimilarStrings`, `FormatSize`, `FormatUptime`.
- **SSH config editing:** `addToSSHConfig` and `removeFromSSHConfig` against golden files, including `Match` blocks, comments, multi-host lines and `Include`. This is the code most likely to damage user files.
- **Crypto and config:** round-trip encryption, legacy plaintext, wrong key, and profile create/read/delete under `t.TempDir()` with `HOME` overridden.
- **Client:** `httptest.Server` fixtures replaying real PVE JSON for auth, cluster resources and task status. This also exercises `ProxmoxClientInterface`.

Fixing P0 #1 and #2 together with their tests would make a good first PR.

### 24. CI
- `.github/workflows/build-test-release.yml` pins `go-version: '1.21'`, but `go.mod` says `go 1.23.0` / `toolchain go1.23.11`. Today it only works because of `GOTOOLCHAIN=auto` downloading 1.23 on every run. Use `go-version-file: go.mod`.
- Add `golangci-lint` (with the `errcheck`, `govet`, `staticcheck`, `ineffassign` and `unused` linters) and `govulncheck`. Several unchecked `Fprintf` errors and the dead code above would have been flagged.
- Run tests with `-race`, since the IP worker pool and caches are concurrent.
- Add a Windows and macOS build matrix entry, or at least `go build` for each GOOS. Windows is shipped but never compiled in CI before release. Note that `syscall.Stdin` in `cmd/ct/create.go` is fine on Windows, but `crypto`'s `user.Current().Uid` is a SID there, which is worth one test.

### 25. Release and distribution
- The Dockerfile build doesn't pass `-ldflags -X main.version=…`, so `docker run ghcr.io/…/prox version` prints `dev`. Pass `VERSION` as a build arg from the metadata action.
- `FROM alpine:latest` is unpinned. Pin a version, or use `gcr.io/distroless/static:nonroot`. The binary is static (`CGO_ENABLED=0`), so it only needs CA certificates.
- Release assets have no checksums or signatures. Consider GoReleaser, which gives checksums, SBOMs, a Homebrew tap and deb/rpm packages, plus cosign signing.
- The "create GHCR package namespace" step builds and pushes a throwaway `:init` image. That is generally unnecessary with `GITHUB_TOKEN` and `packages: write`, and could be dropped.

---

## Feature ideas (Additive, for later)

Rough ordering by value to users:

1. **API token auth** (P1 #9). This is the most requested thing for automation tools like this.
2. **`--output json|yaml|table|wide` everywhere**, as a single persistent flag. Keep the existing `--json` as an alias so nothing breaks. `ct describe`, `node list`, `status`, `config list` and `ct templates` have no JSON today.
3. **Snapshots:** `vm snapshot create/list/rollback/delete`, plus the same for `ct`. This is a very common daily operation that is missing.
4. **`ct` parity with `vm`:** `ct clone`, `ct migrate`, `ct edit`, and `ct reboot`.
5. **`vm create`** from a cloud-init template (clone + set `ipconfig0`/`sshkeys`/`ciuser` + resize + start) in one command.
6. **`prox ssh <name> --connect`** to exec `ssh` directly without editing `~/.ssh/config`, and/or `--print` to output the `ssh` command line.
7. **`prox vm console` / `ct enter`**, wrapping `pct enter` over SSH to the node, or a VNC/serial proxy.
8. **Bulk operations by tag:** `prox vm start --tag web`, with a confirmation prompt that lists the targets.
9. **Download templates:** `prox ct templates --available` and `prox ct templates pull debian:12` using `aplinfo` / `/nodes/{n}/aplinfo`.

---

## Suggested sequencing

| PR | Contents | Risk |
|---|---|---|
| 1 | P0 #1 (template parsing) and #2 (stdin keys), with tests | Safe |
| 2 | P0 #5 (status env vars), #6 (ssh N/A), mojibake, `.gitignore`, CI Go version | Safe |
| 3 | P0 #3 (exit codes), `SilenceUsage`, errors to stderr, spinner to stderr | Low. Release-note the exit codes. |
| 4 | P0 #4 (`--node` wiring), #7 (ct create node) | Low |
| 5 | P1 #8 (crypto: explicit errors, fingerprint check, versioned prefix) | Safe for existing files |
| 6 | P1 #12 (stop fabricating disk numbers) | JSON values change. Release-note it. |
| 7 | P1 #11 (ssh config safety: Match blocks, atomic write, backup, IPv6) | Safe |
| 8 | P1 #9 API tokens, #10 TLS options | Additive |
| 9 | P2 #19 client refactor with an `httptest` suite | Safe, behind tests |

PRs 1–3 alone fix every bug a typical user would notice, without changing anything they depend on.
