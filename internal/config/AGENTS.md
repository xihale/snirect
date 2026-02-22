# internal/config Package

Configuration loading, default management, and rule pattern matching.

## OVERVIEW
Manages the lifecycle of Snirect's configuration: loading TOML files with sensible defaults, and providing access to effective settings. Also generates embedded default configuration.

## WHERE TO LOOK
| File | Purpose |
|------|---------|
| config.go | Core structs: `Config`, `Rules`, `AlterHostname`, `CertVerify`, `Hosts` |
| loader.go | `LoadConfig` function: loads config file, applies defaults, validates |
| match.go | `MatchPattern` function: wildcard domain matching (`*`, `$`, prefixes) |
| update.go | Config update handling, merging fetched rules with overrides |
| defaults.go | Default configuration constants and fallback values |
| defaults_generated.go | Embedded default files via `//go:embed` (DO NOT EDIT) |
| loader_test.go | Tests for config loading and merging |
| match_test.go | Tests for pattern matching |
| tools/gendefaults/main.go | Code generation tool that creates `defaults_generated.go` |

## CONVENTIONS
- Three-tier rule precedence: **user** > **fetched** > **default**.
- Config uses direct unmarshaling; omitted fields retain compile-time defaults.
- Configuration stored in TOML (`config.toml`, `rules.toml`).
- The special marker `__AUTO__` disables fetched rules for a specific entry, reverting to program defaults.
- Pattern matching: `*example.com` (suffix), `example*` (prefix), `$example.com` (exact), `#` comments.
- Code generation: run `go generate ./internal/config` to rebuild defaults after changes; **DO NOT EDIT** generated file.
- Errors include context with `%w` wrapping.
- Upstream rate limiting: `UpdateConfig.UpstreamRateLimit` limits HTTP requests per second from the upstream client (0 = unlimited). Used for rules and update fetching.

## ANTI-PATTERNS
- **DO NOT EDIT** `defaults_generated.go`. Modify source TOML and regenerate.
- **NEVER leave maps nil** during merge; always `make(map)` to avoid panics.
- Global: MUST NOT close response bodies early (relevant in upstream client).
- See root AGENTS.md for full list.

## TESTING
```bash
go test ./internal/config
```
Benchmarks: consider adding if patterns become heavy.