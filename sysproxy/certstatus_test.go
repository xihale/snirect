package sysproxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// makeSelfSignedCA writes a fresh self-signed root CA to a temp file and returns
// its path. Used to exercise the cert-status code path without touching the real
// system trust store.
func makeSelfSignedCA(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Snirect Test Root CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	p := filepath.Join(t.TempDir(), "root.crt")
	if err := os.WriteFile(p, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	return p
}

// TestIsCertInstalled_NotInSystemPool asserts the check returns false for a CA
// that is valid but not present in the system trust store. This guards against
// the old bug where the check always failed for an unrelated reason (re-exec of
// a missing subcommand); here we ensure a *correct* false negative.
func TestIsCertInstalled_NotInSystemPool(t *testing.T) {
	caPath := makeSelfSignedCA(t)
	if isCertInstalled(caPath) {
		t.Fatal("freshly generated CA must not be considered installed in system pool")
	}
}

// TestGetCertFingerprint_RoundTrip ensures the fingerprint helper still works
// after the refactor (it is still used by the macOS path).
func TestGetCertFingerprint_RoundTrip(t *testing.T) {
	caPath := makeSelfSignedCA(t)
	fp1, err := GetCertFingerprint(caPath)
	if err != nil {
		t.Fatalf("GetCertFingerprint: %v", err)
	}
	data, _ := os.ReadFile(caPath)
	fp2, err := GetCertFingerprintFromPEM(data)
	if err != nil {
		t.Fatalf("GetCertFingerprintFromPEM: %v", err)
	}
	if fp1 != fp2 {
		t.Fatalf("fingerprint mismatch: %s != %s", fp1, fp2)
	}
}

// TestKDEProxyConfigPath_XDG verifies the KDE config path honors XDG_CONFIG_HOME
// so status detection can be redirected in tests/environments that set it.
func TestKDEProxyConfigPath_XDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdgtest")
	if got := kdeProxyConfigPath(); got != "/tmp/xdgtest/kioslaverc" {
		t.Fatalf("kdeProxyConfigPath = %q, want /tmp/xdgtest/kioslaverc", got)
	}
}
