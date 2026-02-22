package interfaces

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"

	"snirect/internal/config"
	"snirect/internal/tlsutil"
)

// CertificateManager manages root CA and signs leaf certificates.
type CertificateManager interface {
	GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error)
	GetRootCACertPEM() []byte
	Close() error
}

// Resolver resolves hostnames to IP addresses with caching.
type Resolver interface {
	Resolve(ctx context.Context, host string, clientIP net.IP) (string, error)
	Invalidate(host string)
	Close() error
}

// HTTPClient performs HTTP requests and file downloads.
type HTTPClient interface {
	Get(ctx context.Context, url string) (*http.Response, error)
	DownloadFile(ctx context.Context, url, destPath string) error
}

// CertVerifier verifies TLS certificates with zero-trust hardening.
type CertVerifier interface {
	VerifyCert(conn tlsutil.TLSConnection, host, targetSNI string, policy config.CertPolicy, sec config.SecurityConfig) bool
}
