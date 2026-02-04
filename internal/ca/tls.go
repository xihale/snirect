package ca

import (
	"crypto/tls"
	"encoding/pem"
)

func (cm *CertManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
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
	derBytes, priv, err := cm.Sign([]string{host})
	if err != nil {
		return nil, err
	}

	cert := &tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  priv,
	}

	cm.certCache.Store(host, cert)
	return cert, nil
}

func (cm *CertManager) GetRootCACertPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cm.RootCert.Raw})
}
