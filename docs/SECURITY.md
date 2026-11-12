# Security Features

## Encrypted Configuration

Prox automatically encrypts sensitive credentials (username & password) in the config file.

### How It Works
- AES-256-GCM encryption
- System & user bound key derivation (non-portable)
- Legacy plaintext auto-detected and migrated
- 600 file permissions enforced
- URL left in plaintext (non-sensitive, aids troubleshooting)

### Commands
```bash
prox config migrate   # migrate legacy plaintext
prox config setup -u root@pam -p secret -l https://your-proxmox:8006
prox config read      # decrypted (masked) view
```

### Key Features
1. Automatic migration of old configs
2. Transparent use (encrypt/decrypt behind API)
3. Key fingerprint stored for verification
4. No extra master password required

### Example
Before:
```
username=root@pam
password=mypassword
url=https://192.168.1.230:8006
```
After:
```
username=jeLxnYvjDpJMAM4VYHB/RMQZcq8WjVdzuDlqSsclgZ9Ls1LW
password=01HltxdSiSFDTbITuddlFvSTVgCwcAVtK3tKJcCHx6dyroTONJlKiXb1
url=https://192.168.1.230:8006
# Encrypted config - key fingerprint: f4d78a56c2cd94ee
```

### Considerations
- Config is host/user specific (copying encrypted file elsewhere will fail)
- No manual key export/import yet (future feature)
- Keep filesystem permissions intact

### What this protects against
The encryption key is derived from your user ID, username, hostname and OS. It is
not protected by a password, so treat this as **obfuscation at rest**, not as a
secret store:

- It stops credentials being read by casual inspection, from backups, or from a
  config file copied to another machine.
- It does **not** protect against anything that can run code as your user on
  this machine, because that can derive the same key.

For automation, prefer environment variables (`PROX_URL`, `PROX_USER`,
`PROX_PASSWORD`) supplied by your CI secret store.

### If decryption fails
Because the key includes the hostname, renaming the machine (or macOS changing
its hostname on a new network) makes saved credentials unreadable. prox reports
this with the stored and current key fingerprints. Re-enter your credentials to
fix it:

```bash
prox config create <profile> -u root@pam -p secret -l https://your-proxmox:8006
```

---
For architecture context see ARCHITECTURE.md.
