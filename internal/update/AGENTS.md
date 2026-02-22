# internal/update Package

Update management for community rules and binary self-updates.

## OVERVIEW
Periodically checks for updates: refreshes community rules from Cealing-Host and downloads new Snirect releases from GitHub. Handles verification, installation, and rollback.

## WHERE TO LOOK
| File | Purpose |
|------|---------|
| manager.go | `UpdateManager` type: check for updates, download, verify, apply; background tasks |
| manager_test.go | Unit tests for version comparison, update workflow, error handling |
| rules_test.go | Tests for converting fetched JSON rules to TOML format |

## CONVENTIONS
- Fetches rules JSON from a configurable URL; converts to TOML and writes to user config.
- Self-update uses GitHub releases API; downloads asset matching current OS/arch.
- Checksums (SHA256) verified before applying updates.
- Version comparison uses semver; pre-releases ignored by default.
- Update checks can run in background; logs at Info level.
- Errors are wrapped with context; retries with backoff.

## ANTI-PATTERNS
- MUST verify checksum before installing binary updates.
- DO NOT apply partial updates; atomic replace on successful download.
- Follow global anti-patterns regarding error handling and resource cleanup.

## TESTING
```bash
go test ./internal/update
```
Mock network interactions are used; see test files for setup.