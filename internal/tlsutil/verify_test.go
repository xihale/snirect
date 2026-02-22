package tlsutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"snirect/internal/config"
	"testing"
	"time"
)

func TestMatchHostname(t *testing.T) {
	cert := &x509.Certificate{
		DNSNames: []string{"example.com", "*.example.org"},
	}
	tests := []struct {
		name     string
		hostname string
		policy   config.CertPolicy
		want     bool
	}{
		{name: "Strict - Match", hostname: "example.com", policy: config.CertPolicy{Enabled: true, Strict: true}, want: true},
		{name: "Strict - Wildcard Match", hostname: "foo.example.org", policy: config.CertPolicy{Enabled: true, Strict: true}, want: true},
		{name: "Strict - No Match", hostname: "other.com", policy: config.CertPolicy{Enabled: true, Strict: true}, want: false},
		{name: "Loose - Match exact", hostname: "example.com", policy: config.CertPolicy{Enabled: true, Strict: false}, want: true},
		{name: "Loose - Match sub", hostname: "sub.example.com", policy: config.CertPolicy{Enabled: true, Strict: false}, want: true},
		{name: "Loose - No Match", hostname: "google.com", policy: config.CertPolicy{Enabled: true, Strict: false}, want: false},
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

func TestMatchAllowedPattern(t *testing.T) {
	tests := []struct {
		pattern, host string
		strict        bool
		want          bool
	}{
		{"*.example.com", "test.example.com", false, true},
		{"*.example.com", "test.example.com", true, false},
		{"example.com", "example.com", false, true},
		{"example.com", "example.com", true, true},
	}
	for _, tt := range tests {
		got := matchAllowedPattern(tt.pattern, tt.host, tt.strict)
		if got != tt.want {
			t.Errorf("matchAllowedPattern(%q,%q,%v)=%v; want %v", tt.pattern, tt.host, tt.strict, got, tt.want)
		}
	}
}

func TestVerifyCert_TimeValidity(t *testing.T) {
	now := time.Now()
	sec := config.SecurityConfig{ValidateChain: false, CheckEKU: false, CheckValidity: true}
	policy := config.CertPolicy{Enabled: true}
	validCert, _, err := generateTestCert(true, false, now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("gen validCert: %v", err)
	}
	connValid := &mockConn{state: buildConnState(validCert)}
	if !VerifyCert(connValid, "test.example.com", "", policy, sec) {
		t.Error("valid cert should pass")
	}
	expiredCert, _, err := generateTestCert(true, false, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("gen expired: %v", err)
	}
	connExpired := &mockConn{state: buildConnState(expiredCert)}
	if VerifyCert(connExpired, "test.example.com", "", policy, sec) {
		t.Error("expired cert should fail")
	}
}

func TestVerifyCert_EKU(t *testing.T) {
	now := time.Now()
	sec := config.SecurityConfig{ValidateChain: false, CheckEKU: true, CheckValidity: true}
	policy := config.CertPolicy{Enabled: true}
	withEKU, _, err := generateTestCert(true, false, now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("gen withEKU: %v", err)
	}
	connWith := &mockConn{state: buildConnState(withEKU)}
	if !VerifyCert(connWith, "test.example.com", "", policy, sec) {
		t.Error("cert with serverAuth EKU should pass")
	}
	noEKU, _, err := generateTestCert(false, false, now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("gen noEKU: %v", err)
	}
	connWithout := &mockConn{state: buildConnState(noEKU)}
	if VerifyCert(connWithout, "test.example.com", "", policy, sec) {
		t.Error("cert without serverAuth EKU should fail")
	}
}

func TestVerifyCert_AllowedStrict(t *testing.T) {
	now := time.Now()
	baseSec := config.SecurityConfig{ValidateChain: false, CheckEKU: false, CheckValidity: true}
	policyWildcard := config.CertPolicy{Enabled: true, Allowed: []string{"*.example.com"}}
	policyExact := config.CertPolicy{Enabled: true, Allowed: []string{"test.example.com"}}
	cert := generateTestCertHelper(now, "test.example.com")
	conn := &mockConn{state: buildConnState(cert)}
	if !VerifyCert(conn, "test.example.com", "", policyWildcard, baseSec) {
		t.Error("wildcard match should pass when allowed_strict=false")
	}
	strictSec := baseSec
	strictSec.AllowedStrict = true
	if VerifyCert(conn, "test.example.com", "", policyWildcard, strictSec) {
		t.Error("wildcard match should fail when allowed_strict=true")
	}
	if !VerifyCert(conn, "test.example.com", "", policyExact, strictSec) {
		t.Error("exact allowed match should pass in strict mode")
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
	sec := config.SecurityConfig{ValidateChain: true, MinChainLength: 2}
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
	if err := verifyCertificateChain(leafCert, state1, sec, roots); err == nil {
		t.Error("chain length 1 should fail min length check")
	}
	state2 := buildConnState(leafCert, caCert)
	if err := verifyCertificateChain(leafCert, state2, sec, roots); err != nil {
		t.Errorf("valid chain failed: %v", err)
	}
}

func TestVerifyCert_EndToEnd(t *testing.T) {
	now := time.Now()
	sec := config.SecurityConfig{ValidateChain: false, CheckEKU: true, CheckValidity: true}
	policy := config.CertPolicy{Enabled: true, Strict: false}
	cert, _, err := generateTestCert(true, false, now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("gen cert: %v", err)
	}
	conn := &mockConn{state: buildConnState(cert)}
	if !VerifyCert(conn, "test.example.com", "", policy, sec) {
		t.Error("valid cert should pass end-to-end")
	}
}
