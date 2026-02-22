package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"sync"
	"testing"
	"time"
)

func TestNewCertificateManager_GeneratesNew(t *testing.T) {
	tmpDir := t.TempDir()
	caCertPath := tmpDir + "/root.crt"
	caKeyPath := tmpDir + "/root.key"

	cm, err := NewCertificateManager(caCertPath, caKeyPath)
	if err != nil {
		t.Fatalf("NewCertificateManager failed: %v", err)
	}
	defer cm.Close()

	if cm.RootCert == nil {
		t.Error("RootCert is nil")
	}
	if cm.RootKey == nil {
		t.Error("RootKey is nil")
	}

	// Check files exist
	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		t.Error("CA cert file not created")
	}
	if _, err := os.Stat(caKeyPath); os.IsNotExist(err) {
		t.Error("CA key file not created")
	}
}

func TestSignLeafCert_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	caCertPath := tmpDir + "/root.crt"
	caKeyPath := tmpDir + "/root.key"

	cm, err := NewCertificateManager(caCertPath, caKeyPath)
	if err != nil {
		t.Fatalf("NewCertificateManager failed: %v", err)
	}
	defer cm.Close()

	hosts := []string{"example.com"}
	certDER, keyPEM, err := cm.SignLeafCert(hosts)
	if err != nil {
		t.Fatalf("SignLeafCert failed: %v", err)
	}
	if len(certDER) == 0 {
		t.Error("cert DER empty")
	}
	if keyPEM == nil {
		t.Error("key PEM nil")
	}

	// Parse cert from DER
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	if len(cert.DNSNames) == 0 {
		t.Error("cert has no DNSNames")
	}
	if cert.DNSNames[0] != hosts[0] {
		t.Fatalf("cert DNSName %s, want %s", cert.DNSNames[0], hosts[0])
	}
}

// TestSignLeafCert_MultipleHosts tests signing a certificate with multiple SANs.
func TestSignLeafCert_MultipleHosts(t *testing.T) {
	tmpDir := t.TempDir()
	caCertPath := tmpDir + "/root.crt"
	caKeyPath := tmpDir + "/root.key"

	cm, err := NewCertificateManager(caCertPath, caKeyPath)
	if err != nil {
		t.Fatalf("NewCertificateManager failed: %v", err)
	}
	defer cm.Close()

	hosts := []string{"example.com", "example.org"}
	certDER, _, err := cm.SignLeafCert(hosts)
	if err != nil {
		t.Fatalf("SignLeafCert failed: %v", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	if len(cert.DNSNames) != 2 {
		t.Fatalf("cert DNSNames count: got %d, want 2", len(cert.DNSNames))
	}
	// Verify both hosts are present (order not guaranteed)
	hasA, hasB := false, false
	for _, name := range cert.DNSNames {
		if name == hosts[0] {
			hasA = true
		}
		if name == hosts[1] {
			hasB = true
		}
	}
	if !hasA || !hasB {
		t.Fatalf("cert DNSNames missing expected entries: %v", cert.DNSNames)
	}
}

// TestLoadCA_ECDSASuccess tests loading a CA with an EC private key.
func TestLoadCA_ECDSASuccess(t *testing.T) {
	// Generate EC key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("EC key generation failed: %v", err)
	}

	// Create self-signed CA certificate using EC key
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "EC Test CA"},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("CreateCertificate failed: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	// Marshal EC private key to PEM
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey failed: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	// Load into CertificateManager
	cm := &CertificateManager{}
	if err := cm.LoadCA(certPEM, keyPEM); err != nil {
		t.Fatalf("LoadCA failed: %v", err)
	}
	if cm.RootCert == nil || cm.RootKey == nil {
		t.Error("RootCert or RootKey not set")
	}
}

// TestLoadCA_UnknownKeyType tests that LoadCA returns an error for an unsupported key type.
func TestLoadCA_UnknownKeyType(t *testing.T) {
	// Obtain a valid certificate (RSA) from a generated CA
	tmpDir := t.TempDir()
	caCertPath := tmpDir + "/root.crt"
	caKeyPath := tmpDir + "/root.key"
	cmGen, err := NewCertificateManager(caCertPath, caKeyPath)
	if err != nil {
		t.Fatalf("NewCertificateManager failed: %v", err)
	}
	defer cmGen.Close()
	certPEM := cmGen.GetRootCACertPEM()

	// Create a fake PEM block with unknown key type
	unknownBlock := &pem.Block{Type: "UNKNOWN_KEY", Bytes: []byte("fake key data")}
	keyPEM := pem.EncodeToMemory(unknownBlock)

	cm := &CertificateManager{}
	err = cm.LoadCA(certPEM, keyPEM)
	if err == nil {
		t.Error("LoadCA succeeded with unknown key type, expected error")
	}
}

// TestGetCertificate tests that GetCertificate returns a signed leaf certificate for the given SNI.
func TestGetCertificate(t *testing.T) {
	tmpDir := t.TempDir()
	caCertPath := tmpDir + "/root.crt"
	caKeyPath := tmpDir + "/root.key"
	cm, err := NewCertificateManager(caCertPath, caKeyPath)
	if err != nil {
		t.Fatalf("NewCertificateManager failed: %v", err)
	}
	defer cm.Close()

	hi := &tls.ClientHelloInfo{ServerName: "example.com"}
	cert, err := cm.GetCertificate(hi)
	if err != nil {
		t.Fatalf("GetCertificate failed: %v", err)
	}
	if cert == nil {
		t.Fatal("cert is nil")
	}
	if len(cert.Certificate) == 0 {
		t.Error("certificate has no bytes")
	}

	// Second call should hit cache
	cert2, _ := cm.GetCertificate(hi)
	if cert2 != cert {
		t.Error("cached certificate not the same pointer")
	}
}

// TestCertificateManager_Close tests that Close stops the cleanup routine without panic.
func TestCertificateManager_Close(t *testing.T) {
	tmpDir := t.TempDir()
	caCertPath := tmpDir + "/root.crt"
	caKeyPath := tmpDir + "/root.key"

	cm, err := NewCertificateManager(caCertPath, caKeyPath)
	if err != nil {
		t.Fatalf("NewCertificateManager failed: %v", err)
	}
	// Close should not panic
	if err := cm.Close(); err != nil {
		t.Error("Close returned error:", err)
	}
}

func TestLoadCA_Success(t *testing.T) {
	tmpDir := t.TempDir()
	caCertPath := tmpDir + "/root.crt"
	caKeyPath := tmpDir + "/root.key"

	// Generate a CA first
	cm1, err := NewCertificateManager(caCertPath, caKeyPath)
	if err != nil {
		t.Fatalf("NewCertificateManager failed: %v", err)
	}
	defer cm1.Close()

	certPEM := cm1.GetRootCACertPEM()
	keyPEM := encodePrivateKeyPEM(cm1.RootKey)

	// Load into a new CertificateManager
	cm2 := &CertificateManager{}
	if err := cm2.LoadCA(certPEM, keyPEM); err != nil {
		t.Fatalf("LoadCA failed: %v", err)
	}
	if cm2.RootCert == nil {
		t.Error("RootCert not loaded")
	}
	if cm2.RootKey == nil {
		t.Error("RootKey not loaded")
	}
}

func TestLoadCA_InvalidPEM(t *testing.T) {
	cm := &CertificateManager{}
	err := cm.LoadCA([]byte("invalid"), []byte("also invalid"))
	if err == nil {
		t.Error("LoadCA succeeded with invalid PEM, expected error")
	}
}

func encodePrivateKeyPEM(key interface{}) []byte {
	var der []byte
	var typ string
	switch k := key.(type) {
	case *rsa.PrivateKey:
		typ = "RSA PRIVATE KEY"
		der = x509.MarshalPKCS1PrivateKey(k)
	case *ecdsa.PrivateKey:
		typ = "EC PRIVATE KEY"
		var err error
		der, err = x509.MarshalECPrivateKey(k)
		if err != nil {
			return nil
		}
	default:
		return nil
	}
	block := &pem.Block{Type: typ, Bytes: der}
	return pem.EncodeToMemory(block)
}

// TestLoadCA_KeyCertMismatch tests LoadCA when the key does not match the certificate.
func TestLoadCA_KeyCertMismatch(t *testing.T) {
	// Create two independent CAs in separate directories.
	tmpDir1 := t.TempDir()
	caCertPath1 := tmpDir1 + "/root.crt"
	caKeyPath1 := tmpDir1 + "/root.key"

	cm1, err := NewCertificateManager(caCertPath1, caKeyPath1)
	if err != nil {
		t.Fatalf("NewCertificateManager failed: %v", err)
	}
	defer cm1.Close()
	certPEM := cm1.GetRootCACertPEM()

	tmpDir2 := t.TempDir()
	caCertPath2 := tmpDir2 + "/root.crt"
	caKeyPath2 := tmpDir2 + "/root.key"

	cm2, err := NewCertificateManager(caCertPath2, caKeyPath2)
	if err != nil {
		t.Fatalf("NewCertificateManager second failed: %v", err)
	}
	defer cm2.Close()
	keyPEM := encodePrivateKeyPEM(cm2.RootKey)

	// Try to load cert from first CA with key from second CA (should fail)
	cm := &CertificateManager{}
	err = cm.LoadCA(certPEM, keyPEM)
	if err == nil {
		t.Error("LoadCA succeeded with mismatched key/cert, expected error")
	}
}

// TestSignLeafCert_Concurrent tests that SignLeafCert is safe for concurrent use.
func TestSignLeafCert_Concurrent(t *testing.T) {
	tmpDir := t.TempDir()
	caCertPath := tmpDir + "/root.crt"
	caKeyPath := tmpDir + "/root.key"

	cm, err := NewCertificateManager(caCertPath, caKeyPath)
	if err != nil {
		t.Fatalf("NewCertificateManager failed: %v", err)
	}
	defer cm.Close()

	// Launch multiple concurrent signing operations
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			hosts := []string{fmt.Sprintf("test%d.example.com", idx)}
			certDER, keyPEM, err := cm.SignLeafCert(hosts)
			if err != nil {
				t.Errorf("SignLeafCert failed: %v", err)
				return
			}
			if len(certDER) == 0 {
				t.Error("cert DER empty")
			}
			if keyPEM == nil {
				t.Error("key PEM nil")
			}
			// Parse to ensure validity
			_, err = x509.ParseCertificate(certDER)
			if err != nil {
				t.Errorf("ParseCertificate: %v", err)
			}
		}(i)
	}
	wg.Wait()
}
