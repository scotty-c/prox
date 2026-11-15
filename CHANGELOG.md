# Changelog

All notable changes to prox are recorded here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and prox uses [semantic versioning](https://semver.org/). Releases before 0.0.3 predate this file.

## [1.0.0] - Unreleased

1.0.0 is the first release that treats the command line, its output and its exit codes as a stable interface. Getting there meant fixing behaviour that was wrong, so this release has a number of breaking changes. They are listed first, each with what to do about it.

### Upgrading from 0.0.3

#### Things that will stop a script or a setup working

1. **TLS certificates are verified.** Earlier versions never checked the server's certificate. A Proxmox install with its default self-signed certificate now fails with `TLS certificate verification failed`.
   *Do:* trust the Proxmox CA with `--ca-cert /path/to/pve-root-ca.pem` (copy `/etc/pve/pve-root-ca.pem` from any node), or skip verification with `--insecure`. Either can also be saved in the profile (`prox config update --ca-cert ...` / `--insecure`) or set as `PROX_CA_CERT` / `PROX_INSECURE=true`.

2. **VM commands wait for their task.** `vm start`, `vm shutdown` (and `stop`), `vm reboot`, `vm delete` and `vm create --start` used to return as soon as Proxmox accepted the task, so they could report success and fail afterwards. They now wait and report how long it took, as `ct` commands, `vm clone`, `vm migrate`, snapshots and template downloads already did. `vm shutdown` can therefore block on a guest that ignores ACPI shutdown.
   *Do:* add `--no-wait` where you relied on returning immediately; `--timeout 5m` or Ctrl-C stop the wait without stopping the task; `vm shutdown --force` is the hard stop. `-w`/`--wait` is still accepted and changes nothing.

3. **Saved credentials use a new storage format.** Passwords were encrypted with a key anyone running as you could recompute, and renaming the machine made them unreadable. They are now kept in the OS keychain (macOS Keychain, Windows Credential Manager, the Linux Secret Service), or, where there is none, sealed with a random key file in your user config directory. Profiles saved by 0.x keep working and prox tells you once per run how to move them.
   *Do:* run `prox config migrate` (all profiles, or `prox config migrate <profile>`). **A profile that has been saved or migrated by 1.0 cannot be read by 0.x**, so upgrade every machine and copy of prox that shares your profiles. A profile in the keychain is also not available inside a container or on another machine; use `PROX_URL`, `PROX_USER` and `PROX_PASSWORD` there.

4. **Failures exit non-zero, and a prompt that gets no answer is a failure.**
   - Several commands printed an error and still exited 0 (`vm delete`, `vm edit`, `ct templates`, `vm test`, every `config` subcommand). They exit 1 now.
   - A mistyped subcommand (`prox vm bogus`) exits 1 with suggestions instead of printing help and exiting 0.
   - `vm edit` with nothing to change exits 1 (it warned and exited 0).
   - A confirmation (delete, snapshot rollback, `config delete`, `--tag` bulk operations) with stdin closed used to print "cancelled" and exit 0, so a script that forgot `--force` looked successful. It now exits 1 and names the flag.
   - Ctrl-C exits 130.
   *Do:* in scripts, pass `-y`/`--yes` (or `--force` where it already existed) to skip a prompt.

5. **Progress and errors go to stderr; stdout holds only the result.** Messages such as `Retrieving virtual machines...`, spinners, warnings, tips and the one-time notices used to mix into stdout and corrupted piped and `--json` output.
   *Do:* anything that parsed progress text from stdout should read the result instead (or use `-o json`). `--quiet` still silences progress.

6. **Names that match more than one VM or container are refused.** `prox vm delete web` used to act on whichever `web` came first.
   *Do:* use the ID, or `--node` for VMs. Names that match exactly one resource work as before.

7. **`--node` is honoured.** It was accepted on `vm start`, `shutdown`, `delete`, `edit`, `clone` and `describe` but ignored. It now restricts the lookup, and the command fails if the VM is not on that node. `ct create --node X` used to warn and quietly create the container on whichever node held the template; it now fails if the template is not on `X`, and says how to download it there (`prox ct templates pull <template> --node X`).

8. **Disk usage is no longer invented.** When Proxmox did not report disk usage for a VM, `vm list` filled in 20% (running) or 10% (stopped) of its size, and used `max_disk: 1` as a marker. These went into `--json` output. `disk` is now `0` when the usage is unknown (the table shows `N/A`) and `max_disk` is the real size. `vm list --detailed` and `vm describe` ask the QEMU guest agent for real usage when the VM is running.

9. **`prox ssh` writes safer, different entries.**
   - New entries no longer set `StrictHostKeyChecking no` and `UserKnownHostsFile /dev/null`, so a changed host key is reported. `--no-host-key-check` restores the old behaviour for disposable VMs.
   - The default `User` is your local username, not the VM's name.
   - Entries are inserted after the global section of `~/.ssh/config` (they used to go at the very top, which turned your global settings into settings for that one host), and each carries a `# managed by prox` comment.
   Existing entries change only when they are next updated.

10. **`config` commands.** `config read` masks the password (use `--show-password`). `config create` refuses to save a profile without a URL, username and password (it prompts on a terminal). `config delete` takes an optional profile name, asks first, and can delete the default profile. `config migrate` now moves profiles to the new storage (see 3). Profile names containing path separators are rejected.

#### Output you may be matching on

- Result lines were reworded and shortened. `VM 100 start command issued successfully` plus `Task ID: ...` is now `Task started: <id>` followed by `VM web (ID 100) started in 1.2s`. Emoji and decorations were removed.
- Lists are sorted by ID (nodes by name) instead of in the order Proxmox returns them, and tables no longer start with a blank line.
- `config list` is a table (profile, URL, user, where the credentials are stored, which is in use). `prox status` is drawn with the same table renderer. `ct shortcuts` lists what your cluster can download instead of a fixed list.
- With `-o json`/`yaml`, commands that start a task print one object (`task`, `node`, `waited`, `status`, `message`, `seconds`) instead of text.
- Shell completion only offers what a command can act on: `vm start <TAB>` lists stopped VMs, `vm shutdown <TAB>` running ones.
- `vm test` is hidden (it still works); `prox config test` and `prox status` check a connection.

#### Go packages

The packages under `pkg/` are an implementation detail of the command, not a supported library. Their APIs changed substantially and will keep changing.

### Added

**Commands and options**
- `prox tasks`: recent cluster tasks, with `--running`, `--failed`, `--node` and `--upid`. Gives you the task ID for `prox logs`.
- `prox vm create <name> --from <template>`: clone a cloud-init template, set CPU, memory, disk, IP, user and SSH keys, and optionally start it.
- `prox vm console` and `prox ct enter` (a shell inside a container), over SSH to the node.
- `prox vm reboot`, and for containers `ct clone`, `ct migrate`, `ct edit` and `ct reboot`.
- `prox vm snapshot` and `prox ct snapshot`: `list`, `create`, `rollback`, `delete`.
- `ct templates --available` and `ct templates pull <os:version>` to download templates.
- Bulk operations by tag: `prox vm start --tag web`, and the same for shutdown, `ct start` and `ct stop`. They list the targets and ask first.
- `prox ssh --connect`, `--print` and `--prune` (remove entries prox wrote for VMs that no longer exist; asks first and supports `--dry-run`), and a `MANAGED` column in `--list`.
- `prox node list` (alias of `ls`) and `prox --version`.
- `vm clone` and `ct clone` take the new ID as an optional argument and use the next free one without it.
- `ct create` gains `--bridge`, `--ip`, `--gateway`, `--vlan`, `--cpu`, `--id`, `--start`, and picks a storage that exists (`local-lvm`, else the first that can hold containers).
- `--force` on `vm shutdown` and `ct stop` for a hard stop.
- `-y`/`--yes` on every command that asks for confirmation.
- Global `-o/--output table|json|yaml`, `--timeout`, `--insecure` and `--ca-cert`.
- Proxmox API token authentication (`user@realm!name` as the username, the secret as the password).
- `--password-stdin` on `config setup`, `create` and `update`, which keeps the password out of shell history and process listings. URLs are tidied before saving (`https://` added, trailing `/` and `/api2/json` removed).
- `prox config list` shows each profile's URL, user and where its credentials are stored; `prox config migrate` upgrades them.

**Output**
- `vm list` and `ct list` JSON gain `cpu_percent`, `cpu_count`, `uptime_seconds`, `tag_list` and `ip_address`, next to the existing fields, which keep their meaning. `ct templates` gains `size_bytes`; `node describe` gains `cpu_percent`.
- Shell completion for profile names, nodes and VMs/containers, with descriptions, a three-second limit so an unreachable cluster cannot freeze your shell, and state-aware candidates.

**Behaviour**
- Ctrl-C stops whatever prox is doing, not only task waits. A second Ctrl-C ends the process at once.
- The login is kept between runs for profiles stored in the keychain (about 110 minutes, in the same keychain item), so consecutive commands and TAB completion usually skip the login. `PROX_SESSION_CACHE=off` turns it off.
- New environment variables: `PROX_INSECURE`, `PROX_CA_CERT`, `PROX_CREDENTIAL_STORE` (`auto`, `keychain` or `file`) and `PROX_SESSION_CACHE`.
- Errors carry a tip chosen by what went wrong: an unreachable server, rejected credentials, a missing privilege, a certificate problem and a missing configuration are each recognised and reported once.

### Changed

- Proxmox error messages include the reason Proxmox gave (it sits in the HTTP status line, which prox ignored), and long HTML error bodies are truncated.
- An expired login ticket (they last two hours) triggers one fresh login and a retry instead of failing every later request, for example during `prox logs --follow`.
- Waiting for a task uses exponential backoff instead of a tight loop. Looking up VMs, IP addresses and disk details runs concurrently, and `vm describe` makes about a third fewer requests.
- Docker images and CI builds use a patched Go toolchain and carry real version information. CI runs lint, `govulncheck` and the race detector.

### Fixed

- `prox logs`, `--tail` and `--follow` showed only the first 50 lines of a task log.
- `ct create ... debian:12` could not resolve the template; neither could several other distributions. Reading SSH keys from stdin (`--ssh-keys-file -`) never read any keys.
- `prox ssh` damaged `~/.ssh/config`: new entries went above your global settings, and removing an entry also deleted `Match` blocks, comments and top-level lines that followed it. Writes are now atomic, keep file permissions and leave a `.prox.bak`, and follow a symlinked config. `prox ssh` explains when a guest has no IP, and gives a clear error for an ambiguous name instead of a merged one. `prox ssh --delete` works without reaching the cluster.
- `vm describe` ignored `--node`, listed no disks past `scsi3` (or `virtio3`, `sata3`, `ide3`) and no NICs past `net9`, and reported the QEMU agent as disabled when Proxmox returned it as a string. `ct describe` listed mount points and networks in a different order each run, and `node describe` printed its figures twice.
- `vm edit --disk` and `vm create --disk` could pick a CD-ROM or cloud-init drive, or a random disk, as the one to grow; they now follow the boot order. `vm edit --disk` now waits for the resize.
- A saved password that could not be decrypted (after the machine's name changed) was used as if it were the password, producing a confusing 401. Short plain-text passwords that happened to be valid base64 (`admin123`) were mangled.
- Profile handling: deleting the active profile with no default left a dangling pointer; profiles were written non-atomically and briefly world-readable.
- `prox status` ignored `PROX_URL`, `PROX_USER` and `PROX_PASSWORD`, so it reported "no configuration" in CI where every other command worked.
- `ct create` default network and storage no longer fail on ZFS and clustered installs.
- `--quiet` and `-o json` now keep progress messages out of the way.

### Security

- Certificates are verified (see Upgrading, 1).
- Saved credentials are no longer protected by a key derived from your user and host name (see Upgrading, 3). `docs/SECURITY.md` now states what each storage option does and does not protect against; in short, nothing prox can store stops code running as you from reading your credentials, so use an API token supplied through your secret manager for CI.
- `prox ssh` keeps host key checking on by default; `config read` masks passwords; `--profile ../x` can no longer reach files outside the profiles directory; the key file is created with mode 0600 in a 0700 directory, and a key file that other users can read is refused.

### Known limitations

- The Linux Secret Service and Windows Credential Manager storage was exercised only through an in-memory test double; the macOS Keychain path was verified on a real machine, except updating an item that already exists (the case that saves a login ticket). If saving fails, prox logs in on the next run.
- With `--tag`, VMs are processed one at a time, so `--wait` over many VMs takes the sum of their tasks.
- The bash end-to-end suite under `tests/e2e` was updated for the new defaults but has not been run against a cluster since.
