# Docker Usage

## Build Image
```bash
make docker-build
# or
docker build -t prox:latest .
```

## Configure with Environment Variables
Pass connection details as environment variables:
```bash
docker run --rm \
  -e PROX_URL=https://proxmox:8006 \
  -e PROX_USER=admin@pam \
  -e PROX_PASSWORD=secret \
  -e PROX_INSECURE=true \
  prox:latest vm list
```

To verify the Proxmox certificate instead of using `PROX_INSECURE`, mount the
Proxmox CA (a copy of `/etc/pve/pve-root-ca.pem` from any node):
```bash
docker run --rm \
  -e PROX_URL=https://proxmox:8006 \
  -e PROX_USER=admin@pam \
  -e PROX_PASSWORD=secret \
  -e PROX_CA_CERT=/certs/pve-root-ca.pem \
  -v ~/.prox/pve-root-ca.pem:/certs/pve-root-ca.pem:ro \
  prox:latest vm list
```

Keep the password out of your shell history with an env file:
```bash
docker run --rm --env-file ~/.prox/docker.env prox:latest vm list
```

### Why not mount `~/.prox`?
Saved profiles are encrypted with a key derived from your username and hostname.
Both are different inside the container, so a profile created on the host can't
be decrypted there. Use environment variables instead.

## Interactive Shell
```bash
docker run --rm -it --env-file ~/.prox/docker.env --entrypoint /bin/sh prox:latest
```

## Specific Operations
```bash
docker run --rm --env-file ~/.prox/docker.env prox:latest vm describe 100
docker run --rm --env-file ~/.prox/docker.env prox:latest ct templates
```

## Docker Compose
`docker-compose.yml` example:
```yaml
services:
  prox:
    build: .
    env_file: ~/.prox/docker.env
    command: ["vm", "list"]
```
Run:
```bash
docker compose run --rm prox vm list
```

## Multi-Arch Image
Workflow publishes multi-architecture images (amd64, arm64) on tagged releases.

---
See SECURITY.md for credential storage details; ARCHITECTURE.md for internal layout.
