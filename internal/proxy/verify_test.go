package proxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/xihale/snirect/internal/rules"
)

func TestMatchHostname(t *testing.T) {
	cert := &x509.Certificate{
		DNSNames: []string{"example.com", "*.example.org"},
	}
	tests := []struct {
		name     string
		hostname string
		policy   rules.CertPolicy
		want     bool
	}{
		{name: "Strict - Match", hostname: "example.com", policy: rules.CertPolicy{Enabled: true, Strict: true}, want: true},
		{name: "Strict - Wildcard Match", hostname: "foo.example.org", policy: rules.CertPolicy{Enabled: true, Strict: true}, want: true},
		{name: "Strict - No Match", hostname: "other.com", policy: rules.CertPolicy{Enabled: true, Strict: true}, want: false},
		{name: "Loose - Match exact", hostname: "example.com", policy: rules.CertPolicy{Enabled: true, Strict: false}, want: true},
		{name: "Loose - Match sub", hostname: "sub.example.com", policy: rules.CertPolicy{Enabled: true, Strict: false}, want: true},
		{name: "Loose - No Match", hostname: "google.com", policy: rules.CertPolicy{Enabled: true, Strict: false}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchHostname(cert, tt.hostname, tt.policy); got != tt.want {
				t.Errorf("MatchHostname() = %v, want %v", got, tt.want)
			}
		})
	}
}

func generateTestCert(hasEKU bool, isCA bool, expiry time.Time) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "test.example.com"},
		NotBefore:    time.Now().Add(-24 * time.Hour),
		NotAfter:     expiry,
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		DNSNames:     []string{"test.example.com", "*.example.com"},
	}
	if hasEKU {
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	}
	if isCA {
		tmpl.IsCA = true
		tmpl.KeyUsage |= x509.KeyUsageCertSign | x509.KeyUsageCRLSign
		tmpl.BasicConstraintsValid = true
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{}
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}
	return cert, priv, nil
}

func generateCA() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	return generateTestCert(false, true, time.Now().Add(365*24*time.Hour))
}

// generateChain builds a CA and a CA-signed leaf with the given expiry, EKU
// presence, and DNS name, plus a root pool trusting the CA — enough for a
// two-certificate chain that passes verifyConnCert's mandatory validation.
func generateChain(expiry time.Time, hasEKU bool, dnsName string) (leaf, caCert *x509.Certificate, pool *x509.CertPool, err error) {
	caCert, caPriv, err := generateCA()
	if err != nil {
		return nil, nil, nil, err
	}
	leafPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: dnsName},
		NotBefore:    time.Now().Add(-24 * time.Hour),
		NotAfter:     expiry,
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		DNSNames:     []string{dnsName},
	}
	if hasEKU {
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &leafPriv.PublicKey, caPriv)
	if err != nil {
		return nil, nil, nil, err
	}
	leaf, err = x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, nil, err
	}
	pool = x509.NewCertPool()
	pool.AddCert(caCert)
	return leaf, caCert, pool, nil
}

func generateLeaf(caCert *x509.Certificate, caPriv *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	_, leafPriv, err := generateTestCert(true, false, time.Now().Add(30*24*time.Hour))
	if err != nil {
		return nil, nil, err
	}
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "leaf.example.com"},
		NotBefore:    time.Now().Add(-24 * time.Hour),
		NotAfter:     time.Now().Add(30 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"leaf.example.com"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &leafPriv.PublicKey, caPriv)
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}
	return cert, leafPriv, nil
}

type mockConn struct {
	state tls.ConnectionState
}

func (m *mockConn) ConnectionState() tls.ConnectionState { return m.state }
func (m *mockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 443}
}
func (m *mockConn) Close() error                     { return nil }
func (m *mockConn) VerifyHostname(host string) error { return nil }

func buildConnState(certs ...*x509.Certificate) tls.ConnectionState {
	var chains [][]*x509.Certificate
	if len(certs) > 0 {
		chains = append(chains, certs)
	}
	return tls.ConnectionState{
		Version:            tls.VersionTLS12,
		PeerCertificates:   certs,
		VerifiedChains:     chains,
		HandshakeComplete:  true,
		DidResume:          false,
		CipherSuite:        tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		NegotiatedProtocol: "",
		ServerName:         "",
	}
}

func TestHasServerAuthEKU(t *testing.T) {
	now := time.Now()
	noEKU, _, err := generateTestCert(false, false, now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("gen noEKU: %v", err)
	}
	withEKU, _, err := generateTestCert(true, false, now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("gen withEKU: %v", err)
	}
	if hasServerAuthEKU(noEKU) {
		t.Error("cert without EKU should not have serverAuth")
	}
	if !hasServerAuthEKU(withEKU) {
		t.Error("cert with serverAuth EKU not detected")
	}
}

func TestVerifyCert_TimeValidity(t *testing.T) {
	now := time.Now()
	validLeaf, caCert, pool, err := generateChain(now.Add(24*time.Hour), true, "test.example.com")
	if err != nil {
		t.Fatalf("gen valid chain: %v", err)
	}
	policy := rules.CertPolicy{Enabled: true}
	if !verifyConnCert(&mockConn{state: buildConnState(validLeaf, caCert)}, "test.example.com", "", policy, false, pool) {
		t.Error("valid cert should pass")
	}
	expiredLeaf, caCert2, pool2, err := generateChain(now.Add(-24*time.Hour), true, "test.example.com")
	if err != nil {
		t.Fatalf("gen expired chain: %v", err)
	}
	if verifyConnCert(&mockConn{state: buildConnState(expiredLeaf, caCert2)}, "test.example.com", "", policy, false, pool2) {
		t.Error("expired cert should fail")
	}
}

func TestVerifyCert_EKU(t *testing.T) {
	now := time.Now()
	withEKU, caCert, pool, err := generateChain(now.Add(24*time.Hour), true, "test.example.com")
	if err != nil {
		t.Fatalf("gen withEKU chain: %v", err)
	}
	policy := rules.CertPolicy{Enabled: true}
	if !verifyConnCert(&mockConn{state: buildConnState(withEKU, caCert)}, "test.example.com", "", policy, false, pool) {
		t.Error("cert with serverAuth EKU should pass")
	}
	noEKU, caCert2, pool2, err := generateChain(now.Add(24*time.Hour), false, "test.example.com")
	if err != nil {
		t.Fatalf("gen noEKU chain: %v", err)
	}
	if verifyConnCert(&mockConn{state: buildConnState(noEKU, caCert2)}, "test.example.com", "", policy, false, pool2) {
		t.Error("cert without serverAuth EKU should fail")
	}
}

func TestVerifyCert_AllowedList(t *testing.T) {
	now := time.Now()
	policyWildcard := rules.CertPolicy{Enabled: true, Allowed: []string{"*.example.com"}}
	policyExact := rules.CertPolicy{Enabled: true, Allowed: []string{"test.example.com"}}
	cert := generateTestCertHelper(now, "test.example.com")
	conn := &mockConn{state: buildConnState(cert)}
	if !VerifyCert(conn, "test.example.com", "", policyWildcard, false) {
		t.Error("wildcard allowed match should pass")
	}
	if !VerifyCert(conn, "test.example.com", "", policyExact, false) {
		t.Error("exact allowed match should pass")
	}
}

func generateTestCertHelper(now time.Time, cn string) *x509.Certificate {
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    now.Add(-24 * time.Hour),
		NotAfter:     now.Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{cn},
	}
	der, _ := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	cert, _ := x509.ParseCertificate(der)
	return cert
}

func TestVerifyCertificateChain(t *testing.T) {
	caCert, caPriv, err := generateCA()
	if err != nil {
		t.Fatalf("CA gen: %v", err)
	}
	leafCert, _, err := generateLeaf(caCert, caPriv)
	if err != nil {
		t.Fatalf("leaf gen: %v", err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(caCert)
	state1 := buildConnState(leafCert)
	if err := verifyCertificateChain(leafCert, state1, roots); err == nil {
		t.Error("chain length 1 should fail min length check")
	}
	state2 := buildConnState(leafCert, caCert)
	if err := verifyCertificateChain(leafCert, state2, roots); err != nil {
		t.Errorf("valid chain failed: %v", err)
	}
}

func TestVerifyCert_DisabledPolicySkipsAll(t *testing.T) {
	now := time.Now()
	policy := rules.CertPolicy{Enabled: false}

	// Disabled policy should skip all verification
	expiredCert, _, err := generateTestCert(false, false, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("gen expired cert: %v", err)
	}
	if !VerifyCert(&mockConn{state: buildConnState(expiredCert)}, "wrong.example.com", "", policy, false) {
		t.Error("disabled policy should accept any cert")
	}
}
