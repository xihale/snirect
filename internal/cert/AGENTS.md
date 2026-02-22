# internal/cert Package

Certificate authority generation and management for MITM decryption.

## OVERVIEW
Generates and manages the root CA certificate, dynamically signs leaf certificates for intercepted domains, and handles certificate caching and storage on disk.

## WHERE TO LOOK
| File | Purpose |
|------|---------|
| cert_manager.go | CertificateManager implementation: CA loading/generation, certificate issuance, caching |
| ca_test.go | Unit tests for CA creation, certificate signing, and cache behavior |
| tls.go | TLS configuration helpers, e.g., GetClientCertificate for mutual TLS |

## CONVENTIONS
- CA stored in config directory as PEM files (ca.crt, ca.key) with restricted permissions.
- On-demand certificate generation with in-memory caching to avoid repeated signing.
- Uses `crypto/x509` for certificate creation and validation.
- Errors include context; cryptographic operations may be slow and are logged at appropriate levels.

## ANTI-PATTERNS
- MUST protect CA private key; ensure proper file permissions (0600 for key).
- MUST NOT expose CA certificate to untrusted clients outside the proxy context.
- Follow root guidelines for error handling and resource cleanup (e.g., closing files).

## TESTING
```bash
go test ./internal/cert
```
