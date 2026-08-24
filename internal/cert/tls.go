package cert

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/xihale/snirect/internal/logger"
)

// renewBefore is how long before NotAfter a cached leaf is re-minted, to
// cover clock skew when proxying for a device whose clock differs from ours.
const renewBefore = 5 * time.Minute

func (cm *CertificateManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	host := hello.ServerName
	if host == "" {
		// If SNI is missing, fallback to a generic name to avoid "no certificates configured" error.
		// This happens for direct IP access or legacy clients.
		host = "snirect.local"
	}

	if cached, ok := cm.certCache.Load(host); ok {
		cert := cached.(*tls.Certificate)
		// The hourly cleanup ticker is monotonic-clock driven and does not
		// advance across system suspend, so an expired leaf can outlive it.
		// Re-check on every hit instead of serving a stale certificate.
		// A nil Leaf cannot happen via Store below; treat it as expired.
		if cert.Leaf != nil && time.Now().Before(cert.Leaf.NotAfter.Add(-renewBefore)) {
			return cert, nil
		}
		cm.certCache.Delete(host)
	}

	// Generate
	derBytes, priv, err := cm.SignLeafCert([]string{host})
	if err != nil {
		return nil, err
	}

	leafCert, _ := x509.ParseCertificate(derBytes)
	if leafCert != nil {
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

	cm.certCache.Store(host, cert)
	return cert, nil
}

func (cm *CertificateManager) GetRootCACertPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cm.RootCert.Raw})
}
