# internal/tlsutil Package

TLS certificate verification utilities for secure connections with custom policies.

## OVERVIEW
Implements custom TLS verification logic beyond Go's standard hostname checking. Supports strict and loose verification, chain validation, EKU checks, and validity period enforcement.

## WHERE TO LOOK
| File | Purpose |
|------|---------|
| verify.go | `Verify` function performing certificate chain and hostname validation with configurable strictness |
| verify_test.go | Unit tests for verification scenarios (valid, expired, mismatched, self-signed, etc.) |

## CONVENTIONS
- Accepts `InsecureSkipVerify=true` but performs manual verification, allowing fine-grained control.
- Supports strict (exact hostname) and loose (wildcard/subdomain) matching modes.
- Checks certificate EKU for `serverAuth`, maximum hop count (usually 0), and validity period (notBefore/notAfter).
- Errors include specific failure reason (e.g., "hostname mismatch", "expired", "unsupported EKU").

## ANTI-PATTERNS
- NEVER bypass verification without performing comprehensive chain and hostname checks.
- MUST validate the entire certificate chain to a trusted root.
- Follow root anti-patterns regarding TLS, especially the requirement to pair `InsecureSkipVerify` with manual verification.

## TESTING
```bash
go test ./internal/tlsutil
```
