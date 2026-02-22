# internal/upstream Package

HTTP client for update checks and rule synchronization with configurable rate limiting.

## OVERVIEW
Provides a reusable HTTP client for fetching remote resources (updates, community rules). Features connection pooling, rate limiting, and manual TLS verification consistent with proxy security model.

## WHERE TO LOOK
| File | Purpose |
|------|---------|
| client.go | UpstreamClient implementation: request execution, connection reuse, rate limiting |
| upstream_test.go | Tests for client behavior including rate limit, redirects, TLS error handling |

## CONVENTIONS
- Wraps standard `http.Client` with custom Transport and rate limiter.
- Rate limiting via token bucket controlled by `upstream_rate_limit` config.
- Manual TLS verification via `tlsutil.Verify` (bypasses standard VerifyPeerCertificate).
- Logs requests and errors at Info/Debug level with context.

## ANTI-PATTERNS
- MUST close response bodies appropriately to avoid leaks; wrap with connection lifecycle when possible.
- MUST respect configured rate limits to avoid overwhelming upstream servers.
- Follow global anti-patterns regarding error handling and resource cleanup.

## TESTING
```bash
go test ./internal/upstream
```
