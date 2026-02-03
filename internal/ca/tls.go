package ca

import (
	"crypto/tls"
	"encoding/pem"
)

func (cm *CertManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	host := hello.ServerName
	if host == "" {
		// If SNI is still missing, we can't generate a valid cert for "nothing".
		// Returning nil causes "no certificates configured".
		return nil, nil
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
