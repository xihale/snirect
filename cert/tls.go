package cert

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/xihale/snirect/logger"
)

func (cm *CertificateManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	host := hello.ServerName
	if host == "" {
		// If SNI is missing, fallback to a generic name to avoid "no certificates configured" error.
		// This happens for direct IP access or legacy clients.
		host = "snirect.local"
	}

	if cert, ok := cm.certCache.Load(host); ok {
		return cert.(*tls.Certificate), nil
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
	}

	cm.certCache.Store(host, cert)
	return cert, nil
}

func (cm *CertificateManager) GetRootCACertPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cm.RootCert.Raw})
}
