package cert

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/xihale/snirect/internal/logger"
)

// renewBefore is how long before NotAfter a cached leaf is re-minted, to
// cover clock skew when proxying for a device whose clock differs from ours.
const renewBefore = 5 * time.Minute

func normalizeCertHost(serverName string) string {
	if serverName == "" {
		return "snirect.local"
	}
	needsNorm := serverName[len(serverName)-1] == '.'
	if !needsNorm {
		for i := 0; i < len(serverName); i++ {
			if c := serverName[i]; c >= 'A' && c <= 'Z' {
				needsNorm = true
				break
			}
		}
	}
	if !needsNorm {
		return serverName
	}
	norm := strings.ToLower(strings.TrimSuffix(serverName, "."))
	if norm == "" {
		return "snirect.local"
	}
	return norm
}

func isValidCachedCert(cert *tls.Certificate, now time.Time) bool {
	return cert != nil && cert.Leaf != nil && now.Before(cert.Leaf.NotAfter.Add(-renewBefore))
}

func (cm *CertificateManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	host := normalizeCertHost(hello.ServerName)
	now := time.Now()

	if cached, ok := cm.certCache.Load(host); ok {
		cert := cached.(*tls.Certificate)
		// The hourly cleanup ticker is monotonic-clock driven and does not
		// advance across system suspend, so an expired leaf can outlive it.
		// Re-check on every hit instead of serving a stale certificate.
		// A nil Leaf cannot happen via storeCert below; treat it as expired.
		if isValidCachedCert(cert, now) {
			return cert, nil
		}
		cm.deleteCert(host)
	}

	v, err, _ := cm.sf.Do(host, func() (interface{}, error) {
		if cached, ok := cm.certCache.Load(host); ok {
			cert := cached.(*tls.Certificate)
			if isValidCachedCert(cert, time.Now()) {
				return cert, nil
			}
		}

		derBytes, priv, err := cm.SignLeafCert([]string{host})
		if err != nil {
			return nil, err
		}

		leafCert, _ := x509.ParseCertificate(derBytes)
		if leafCert != nil && logger.Client().Enabled(context.Background(), slog.LevelDebug) {
			logger.Client().Debug("signed leaf certificate",
				"host", host,
				"serial", leafCert.SerialNumber,
				"issuer", leafCert.Issuer,
				"dns_names", leafCert.DNSNames,
				"not_before", leafCert.NotBefore,
				"not_after", leafCert.NotAfter,
				"ca_fingerprint", fmt.Sprintf("%x", cm.RootCert.SerialNumber),
			)
		}

		cert := &tls.Certificate{
			Certificate: [][]byte{derBytes, cm.RootCert.Raw},
			PrivateKey:  priv,
			Leaf:        leafCert,
		}

		cm.storeCert(host, cert)
		return cert, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*tls.Certificate), nil
}

func (cm *CertificateManager) GetRootCACertPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cm.RootCert.Raw})
}
