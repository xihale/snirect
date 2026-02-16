# Snirect Project Knowledge Base

**Generated:** 2025-02-15 (commit 7ae5ef9, branch main)

## Overview

Snirect is a cross-platform transparent HTTP/HTTPS proxy that modifies SNI (Server Name Indication) to bypass SNI-based censorship (SNI RST). It operates as a system-level proxy using PAC auto-configuration and performs man-in-the-middle decryption with its own CA. Core written in Go, supports Linux, macOS, Windows.

Key features: SNI modification, PAC-based routing, automatic CA installation, system proxy integration, Firefox-specific certificate handling, optional QUIC (DoQ/H3) support, community rules via Cealing-Host.

## Structure

```
snirect/
├── cmd/snirect/          CLI entry point
├── internal/
│   ├── app/              Install/uninstall and service management
│   ├── ca/               Certificate authority
│   ├── cmd/              All CLI commands (cobra)
│   ├── config/           Configuration loading, merging, defaults generation
│   ├── dns/              DNS resolution (UDP/TCP/TLS/DoH/DoQ)
│   ├── logger/           Structured logging wrapper (log/slog)
│   ├── proxy/            HTTP/HTTPS MITM proxy server
│   ├── sysproxy/         System proxy configuration (platform-specific)
│   ├── tlsutil/          TLS utilities & hostname verification
│   ├── update/           Update management (rules + self-update)
│   └── upstream/         Upstream HTTP client with connection pooling
├── dist/                 Build output (gitignored)
├── magefile.go           Build system (Mage)
├── go.mod                Go 1.25.5, module snirect
└── README.md             User-facing docs
```

## Where to Look

| Concern | Location | Notes |
|---------|----------|-------|
| Build/compile | magefile.go targets | `go run github.com/magefile/mage build` |
| CLI commands | internal/cmd/ | cobra commands; root.go is entry |
| Proxy engine | internal/proxy/proxy.go | MITM, SNI modification, connection handling |
| DNS resolution | internal/dns/ | resolver_base.go (core), backends (std/quic) |
| System proxy | internal/sysproxy/ | Platform-specific settings, Firefox cert |
| Installation | internal/app/ | install.go, platform-specific un/installers |
| Configuration | internal/config/ | loader.go (merge), match.go (rules), defaults generation |
| Logging | internal/logger/logger.go | wrapper around log/slog |
| Updates | internal/update/manager.go | rules sync, self-update |
| Certificate CA | internal/ca/ | CA creation, certificate generation |
| TLS verification | internal/tlsutil/verify.go | hostname verification (strict/loose) |
| Upstream HTTP | internal/upstream/client.go | connection reuse, manual TLS verify |

## Code Map (Key Symbols)

| Symbol | Type | Location | Role |
|--------|------|----------|------|
| ProxyServer | struct | internal/proxy/proxy.go | Main proxy server, Start() loop |
| Config | struct | internal/config/config.go | Global configuration |
| Rules | struct | internal/config/rules.go (shared) | Rule set for host/SNI mapping |
| CertManager | struct | internal/ca/ca.go | Manages root CA and certificates |
| Resolver | struct | internal/dns/resolver_base.go | DNS resolver with caching/preference |
| NewResolver | func | internal/dns/resolver_base.go | Constructor for Resolver |
| Install/Uninstall | func | internal/app/*.go | Platform-specific install/uninstall |
| runProxy | func | internal/cmd/run.go | CLI command entry, orchestrates startup |
| LoadConfig | func | internal/config/loader.go | Loads, merges, validates config |
| MatchPattern | func | internal/config/match.go | Pattern matching for rule domains |
| logger | package | internal/logger/logger.go | Log levels, output, formatting |

## Conventions

### Build
- Uses **Mage** (not Make): `go run github.com/magefile/mage <target>`
- Common targets: `build`, `full` (QUIC), `crossAll`, `updateRules`, `clean`
- Version via git tags: `git describe --tags --abbrev=0`, injected with LDFLAGS
- Build tag `quic` enables QUIC support (smaller default binary)
- No Makefile, no container builds

### Code
- **Naming**: camelCase for functions/vars, PascalCase for types
- **Error handling**: `if err != nil { return fmt.Errorf("context: %w", err) }`
- **Logging**: `logger.Debug/Info/Warn/Error/Fatal(msg, args...)`
- **Tests**: co-located `_test.go` files, table-driven subtests, `t.TempDir()`, no testify
- **Platform-specific**: `_<os>.go` files (darwin, linux, windows)
- **Generated code**: `*_generated.go` marked "DO NOT EDIT"
- **Go version**: 1.25.5
- **No linting** configured (golangci-lint, staticcheck absent)
- **No CI test** step (tests exist but not run in GitHub Actions)

### Configuration
- Three-tier rule system: user > fetched > default
- Override marker `__AUTO__` to disable fetched rules for specific entries
- Presence detection via pointer fields to determine which config keys were set
- Default config embedded via `//go:embed` (generated into Go literals for zero-cost startup)
- Format: TOML (config.toml, rules.toml)

### Development
- `go generate ./...` runs code generators – triggered via mage
- Commit style: concise, no prescribed format
- Branch: main, tags semantic versioning (vX.Y.Z)

## Anti-Patterns

1. **DO NOT EDIT generated files** (`defaults_generated.go`). Modify source TOML and run `go generate`.
2. **NEVER leave maps nil** when merging (loader.go pattern). Use `make(map)` to avoid panics.
3. **MUST NOT close response bodies early**; wrap with connection lifecycle (upstream/client.go).
4. **ALWAYS use InsecureSkipVerify with comprehensive manual verification** (tlsutil.Verify). Bypassing without verification is insecure.
5. **NEVER use unsafe pointers outside platform-specific code** (sysproxy_windows.go). Isolate to FFI only.
6. **SNI stripping (empty SNI) is ignored in direct connections** to avoid TLS failures – only performed in MITM path.
7. **QUIC DNS requires `quic` build tag**; default builds exclude QUIC.

## Commands

```bash
# Build
go run github.com/magefile/mage build      # Standard (~8MB)
go run github.com/magefile/mage full       # With QUIC (~11MB)
go run github.com/magefile/mage updateRules # Sync community rules
go run github.com/magefile/mage clean      # Clean dist/, log.txt, completions/

# Tests (not in CI)
go test ./...
go test -v ./internal/config
go test -bench=. ./internal/dns

# Cross-compile release
go run github.com/magefile/mage crossAll
go run github.com/magefile/mage upx        # Compress binaries (requires upx-ucl)
```

## Notes

- **Security**: Manual TLS verification bypasses standard hostname checks. InsecureSkipVerify + incomplete verification = MITM risk. Do not use for highly sensitive sites.
- **Gemini Issue**: Gemini returns fallback certificate when SNI is modified.
- **No Tests in CI**: Consider adding `go test ./...` to build workflow.
- **Codegen**: After changing config types, run `go generate ./internal/config`.
- **QUIC**: Build with `mage full` for DoQ/H3.
- **Rule Precedence**: user rules > fetched rules > defaults; `__AUTO__` reverts to default for specific entries.
- **Large Files**: DNS resolver (resolver_base.go 594 lines) – see internal/dns/AGENTS.md for detailed design.
