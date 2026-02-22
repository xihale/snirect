# internal/dns Package

DNS resolution subsystem supporting multiple backends with caching and IP preference.

## OVERVIEW
Provides DNS resolution with pluggable backends (UDP/TCP/TLS/DoH/DoQ), caching, and intelligent IP selection (IPv4/IPv6 preference). Central to Snirect's ability to resolve hostnames reliably and efficiently.

## WHERE TO LOOK
| File | Purpose |
|------|---------|
| resolver_base.go | Core resolver, cache, backend interface, orchestration (594 lines) |
| resolver_std.go | Standard UDP/TCP resolver implementation |
| resolver_quic.go | DNS over QUIC (DoQ) backend (requires `quic` build tag) |
| resolver_preference.go | IP preference caching and selection logic |
| resolver_preference_test.go | Unit tests for preference cache |

## CONVENTIONS
- Backend interface `Backend` defines `Resolve(context, string) ([]net.IP, error)`.
- Caching per-query with TTL; cache invalidation on failure.
- Preference system records successful IPs to favor same-family addresses.
- Build tag `quic` enables DoQ support; default builds exclude it.
- Errors include context; common error types defined in package.
- Benchmarks present for performance-critical paths.

## ANTI-PATTERNS
None specific to this package; follow global anti-patterns from root AGENTS.md.

## TESTING
```bash
go test ./internal/dns
go test -bench=. ./internal/dns
```