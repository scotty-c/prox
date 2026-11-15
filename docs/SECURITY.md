# Security Features

## Saved credentials

A profile (`~/.prox/profiles/<name>`) holds the URL, TLS settings and the login. The login is kept one of two ways, chosen when the profile is saved:

| Where | When | What the profile file holds |
|-------|------|-----------------------------|
| **The OS keychain** | A keychain is available: macOS Keychain, Windows Credential Manager, or the Linux Secret Service (GNOME Keyring, KWallet) | `credentials=keychain`, and no secret |
| **A key file** | There is no usable keychain, for example a headless server or an SSH session | `username=enc:v2:...` and `password=enc:v2:...`, sealed with AES-256-GCM |

The key file is a random 256-bit key, created on first use with mode `0600` in a `0700` directory at `prox/key` under your user config directory (`~/.config` on Linux, `~/Library/Application Support` on macOS, `%AppData%` on Windows). It is kept apart from `~/.prox` on purpose, so copying, backing up or committing your profiles does not copy the key that opens them. prox refuses to use a key file that other users can read.

`prox config list` shows how each profile is stored. Set `PROX_CREDENTIAL_STORE` to `keychain` or `file` to force one (`auto`, the default, tries the keychain first and says why when it falls back).

```bash
prox config setup -u root@pam -l https://your-proxmox:8006 --password-stdin
prox config list      # the STORED IN column
prox config migrate   # move profiles saved by older versions
```

### What this protects against, and what it does not

No scheme that prox can use on its own stops code running **as you, while you are logged in**, from reading your credentials: that code can ask the keychain, or read the key file, exactly as prox does. What storing credentials this way does give you:

- **Nothing secret in prox's files.** Reading a profile, a backup of `~/.prox`, a dotfiles repository or a config copied to another machine does not reveal the login (with a key file, the key is not in those files either).
- **A store that is encrypted at rest and tied to your login.** The keychain is encrypted with your login credentials and can be locked.
- **No dependence on the machine's name.** Renaming the computer, or macOS changing its hostname on a new network, no longer makes saved credentials unreadable.

Per platform:

- **macOS:** the item is created through `/usr/bin/security`, which stays on its access list, so any process of yours that runs `security find-generic-password -s prox -a <profile> -w` can read it without a prompt. Passwords are passed to `security` on stdin, never on its command line.
- **Linux:** the Secret Service is unlocked for your whole login session; other programs in that session can ask it.
- **Windows:** Credential Manager protects items with your account (DPAPI); your own processes can read them.
- **Key file:** protects against anyone who can read the profile but not the key file. Anyone who can read both (you, root, malware running as you) can open the credentials.

If that is not enough, do not save the password: use an **API token** with only the privileges you need (it can be revoked on its own), and supply it through your secret manager as `PROX_USER`, `PROX_PASSWORD` and `PROX_URL`, which prox prefers over any profile. This is the right choice for CI.

### Profiles saved by older versions

Earlier versions encrypted the login with a key derived from your user ID, user name, host name and OS. Anything running as you can recompute that key, so it only hid the password from casual inspection, and renaming the machine made it unreadable. Those profiles still work. prox says so (once per run, on stderr) and `prox config migrate` moves them to the keychain or key file. Each profile is read back after it is written, and is left as it was if that fails. A profile that has been moved cannot be read by an older prox.

### If something is missing

- **The key file is gone or replaced:** the credentials sealed with it cannot be recovered, and prox says so rather than create a new key. Save the profile again:
  ```bash
  prox config create <profile> -u root@pam --password-stdin -l https://your-proxmox:8006
  ```
  Back up the key file with your profiles if you want to restore them together.
- **The keychain item was deleted:** the same fix.
- **A locked keychain:** unlock it and run the command again.
- **Old-format profile after a rename:** prox reports the stored and current key fingerprints; save the profile again.

### Other protections
- Profile files are written with mode `600`, through a temporary file and a rename, so a failure never leaves a half-written profile.
- `prox config read` masks the password unless you pass `--show-password`; `prox config list` never shows it.
- TLS certificates are verified by default (see the README).

---
For architecture context see ARCHITECTURE.md.
