package interfaces

import (
	"context"
	"crypto/tls"
	"net"
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
