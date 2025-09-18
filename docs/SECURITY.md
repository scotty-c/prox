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

---
For architecture context see ARCHITECTURE.md.
