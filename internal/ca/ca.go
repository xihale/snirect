package ca

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"sync"
	"time"
)

type CertManager struct {
	RootCert  *x509.Certificate
	RootKey   interface{}
	certCache sync.Map // map[string]*tls.Certificate
}

func NewCertManager(caCertPath, caKeyPath string) (*CertManager, error) {
	cm := &CertManager{}

	// Try loading existing CA
	certPEM, err := os.ReadFile(caCertPath)
	if err == nil {
		keyPEM, err := os.ReadFile(caKeyPath)
		if err == nil {
			if err := cm.loadCA(certPEM, keyPEM); err == nil {
				return cm, nil
			}
		}
	}

	// Generate new CA if loading failed or not found
	if err := cm.generateCA(); err != nil {
		return nil, err
	}

	// Save to disk
	if err := cm.saveCA(caCertPath, caKeyPath); err != nil {
		return nil, err
	}

	return cm, nil
}

func (cm *CertManager) loadCA(certPEM, keyPEM []byte) error {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return errors.New("failed to parse certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}

	block, _ = pem.Decode(keyPEM)
	if block == nil {
		return errors.New("failed to parse key PEM")
	}

	var key interface{}
	if block.Type == "RSA PRIVATE KEY" {
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	} else if block.Type == "EC PRIVATE KEY" {
		key, err = x509.ParseECPrivateKey(block.Bytes)
	} else {
		return errors.New("unknown key type: " + block.Type)
	}

	if err != nil {
		return err
	}

	switch k := key.(type) {
	case *rsa.PrivateKey:
		if cert.PublicKeyAlgorithm != x509.RSA {
			return errors.New("certificate public key algorithm mismatch (expected RSA)")
		}
		pubKey, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return errors.New("certificate public key type mismatch")
		}
		if pubKey.N.Cmp(k.N) != 0 || pubKey.E != k.E {
			return errors.New("private key does not match certificate public key")
		}
	case *ecdsa.PrivateKey:
		if cert.PublicKeyAlgorithm != x509.ECDSA {
			return errors.New("certificate public key algorithm mismatch (expected ECDSA)")
		}
		pubKey, ok := cert.PublicKey.(*ecdsa.PublicKey)
		if !ok {
			return errors.New("certificate public key type mismatch")
		}
		if pubKey.X.Cmp(k.X) != 0 || pubKey.Y.Cmp(k.Y) != 0 {
			return errors.New("private key does not match certificate public key")
		}
	}

	cm.RootCert = cert
	cm.RootKey = key

	// Start cleanup routine
	go cm.cleanupRoutine()

	return nil
}

func (cm *CertManager) generateCA() error {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "Snirect Root CA",
			Organization: []string{"Snirect"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return err
	}

	cm.RootCert = cert
	cm.RootKey = priv
	return nil
}

func (cm *CertManager) saveCA(certPath, keyPath string) error {
	certOut, err := os.Create(certPath)
	if err != nil {
		return err
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: cm.RootCert.Raw}); err != nil {
		return err
	}

	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer keyOut.Close()

	var privBytes []byte
	var privType string
	switch k := cm.RootKey.(type) {
	case *rsa.PrivateKey:
		privBytes = x509.MarshalPKCS1PrivateKey(k)
		privType = "RSA PRIVATE KEY"
	case *ecdsa.PrivateKey:
		privBytes, err = x509.MarshalECPrivateKey(k)
		if err != nil {
			return err
		}
		privType = "EC PRIVATE KEY"
	default:
		return errors.New("unsupported key type for saving")
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: privType, Bytes: privBytes}); err != nil {
		return err
	}
	return nil
}

func (cm *CertManager) Sign(hosts []string) ([]byte, interface{}, error) {
	// Generate leaf key (ECDSA is faster)
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Snirect Proxy"},
		},
		NotBefore:   time.Now().Add(-1 * time.Hour),
		NotAfter:    time.Now().Add(24 * time.Hour), // Short validity for leaf certs
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    hosts,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, cm.RootCert, &priv.PublicKey, cm.RootKey)
	if err != nil {
		return nil, nil, err
	}

	return derBytes, priv, nil
}

func (cm *CertManager) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		cm.certCache.Range(func(key, value interface{}) bool {
			// We can't easily check cert expiry without parsing, but we know we issue them for 24h.
			// A simple strategy is to just clear the cache periodically, or check creation time if we stored it.
			// Since we only store *tls.Certificate, we rely on the fact that if it's in cache, it was generated recently?
			// No, it could stay there forever.

			// Let's parse the leaf if not present (it usually isn't populated by our manual struct construction)
			cert := value.(*tls.Certificate)
			if len(cert.Certificate) > 0 {
				// Parse the first cert in chain (leaf)
				x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
				if err == nil {
					if time.Now().After(x509Cert.NotAfter) {
						cm.certCache.Delete(key)
					}
				}
			}
			return true
		})
	}
}
