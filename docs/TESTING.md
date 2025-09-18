# Testing Overview

## Layers
- Unit tests (Go): validation, parsing, SSH key formats.
- E2E harness (bash in tests/e2e): exercises real Proxmox operations end‑to‑end.

## E2E Flow (Simplified)
1. Optional auto build (stale binary detection)
2. Config verification / leftover resource detection
3. VM operations (list/describe/start/stop/clone/migrate/delete) with readiness polling
4. Container lifecycle (create with SSH keys → describe → start/stop → delete)
5. Migration tests (online/offline, local disk optional)
6. Negative tests (invalid IDs, unknown commands) – exit codes normalized
7. Cleanup / leftover reconciliation

## Features
- Parallel VM + container phases (env: PARALLEL_VM_CT_TESTS)
- Adaptive shutdown (multi-attempt graceful → force)
- Soft-failure suppression for transient status/lock waits
- Retry wrapper for transient API/network issues (env flags)
- SSH key validation (formats: RSA, Ed25519, ECDSA, DSS)

## Key Env Flags (E2E)
| Flag | Purpose |
|------|---------|
| AUTO_BUILD | Rebuild binary if sources newer than existing build |
| PARALLEL_VM_CT_TESTS | Run VM & container phases concurrently |
| WAIT_FOR_VM_IP | Poll for IP assignment before proceeding |
| RETRY_TRANSIENT | Enable retry wrapper for select operations |
| ENABLE_EXTRA_CONTAINER_TESTS | Toggle additional container scenarios |
| AUTO_CLEAN_LEFTOVERS | Auto delete stray test resources from prior runs |
| REUSE_LEFTOVERS | Reuse existing test resources instead of recreating |

## Quick Run
```bash
make build
prox config setup -u admin@pam -p pass -l https://proxmox:8006
cd tests/e2e
./setup.sh setup
./setup.sh run
```

## Adding a New Test
1. Add function in `run_e2e_tests.sh` (keep idempotent if possible)
2. Register it in the ordered execution list
3. Ensure cleanup on failure
4. Prefer describe/state validation after mutating operations

## Soft Failures
Some waits (e.g., lock release) can emit warnings without failing the suite to reduce false negatives; search for `soft_failure` in the script for patterns.

---
For architecture: ARCHITECTURE.md. For security: SECURITY.md.
