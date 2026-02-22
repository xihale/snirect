# internal/proxy Package

HTTP/HTTPS proxy engine with MITM decryption and SNI modification.

## OVERVIEW
Core component handling client connections, CONNECT tunneling, and TLS interception. Distinguishes between direct tunnels and full MITM decryption based on configuration rules.

## WHERE TO LOOK
| File | Purpose |
|------|---------|
| proxy.go | Main ProxyServer implementation, request routing, CONNECT handling, MITM logic |
| state.go | Connection state machine defining session states and transitions |
| proxy_test.go | Unit tests for proxy server behavior including tunnel and MITM |
| integration_test.go | End-to-end integration tests for PAC and certificate endpoints |

## CONVENTIONS
- Uses `sync.Pool` to reuse buffers and reduce GC pressure.
- State-driven connection lifecycle via state package (e.g., state.Connect, state.HTTP, state.Direct, state.MITM).
- Manual TLS verification via `tlsutil.Verify` when `InsecureSkipVerify` is enabled.
- Errors include contextual information with `%w` wrapping.
- Logging via `logger` package for diagnostics.

## ANTI-PATTERNS
- MUST NOT close response bodies early; wrap with connection lifecycle.
- When using `InsecureSkipVerify`, MUST perform comprehensive manual verification (tlsutil.Verify).
- Follow global anti-patterns from root AGENTS.md (unsafe usage, type safety, etc.).

## TESTING
```bash
go test ./internal/proxy
```
