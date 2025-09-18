# Docker Usage

## Build Image
```bash
make docker-build
# or
docker build -t prox:latest .
```

## Run Commands
```bash
docker run --rm prox:latest vm list
```

## Persist Config
Create config on host first:
```bash
prox config setup -u admin@pam -p secret -l https://proxmox:8006
```
Then mount:
```bash
docker run --rm -v ~/.prox:/home/prox/.prox prox:latest vm list
```

## Interactive Shell
```bash
docker run --rm -it -v ~/.prox:/home/prox/.prox prox:latest /bin/sh
```

## Specific Operations
```bash
docker run --rm -v ~/.prox:/home/prox/.prox prox:latest vm describe 100
docker run --rm -v ~/.prox:/home/prox/.prox prox:latest ct templates
```

## Environment Variables (Optional / Less Secure)
```bash
docker run --rm \
  -e PROXMOX_URL=https://proxmox:8006 \
  -e PROXMOX_USER=admin@pam \
  -e PROXMOX_PASS=secret \
  prox:latest vm list
```
(Stored encrypted config file is preferred.)

## Docker Compose
`docker-compose.yml` example:
```yaml
version: '3.8'
services:
  prox:
    build: .
    volumes:
      - ~/.prox:/home/prox/.prox:ro
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
