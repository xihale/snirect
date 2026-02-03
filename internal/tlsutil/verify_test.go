package tlsutil

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"testing"
)

func TestMatchHostname_Vulnerability(t *testing.T) {
	// Scenario: Attacker has a valid cert for "evil.co.uk"
	cert := &x509.Certificate{
		DNSNames: []string{"evil.co.uk"},
		Subject: pkix.Name{
			CommonName: "evil.co.uk",
		},
	}

	// Attacker tries to impersonate "google.co.uk"
	// The naive "last 2 parts" check sees "co.uk" == "co.uk" and allows it.
	target := "google.co.uk"

	if MatchHostname(cert, target, true) {
		t.Fatalf("CRITICAL VULNERABILITY: MatchHostname allowed %s to match cert for %v", target, cert.DNSNames)
	}
}

func TestMatchHostname_Normal(t *testing.T) {
	cert := &x509.Certificate{
		DNSNames: []string{"www.example.com"},
	}

	if !MatchHostname(cert, "www.example.com", true) {
		t.Error("Failed to match exact domain")
	}

	// Current loose logic allows subdomains sharing SLD
	if !MatchHostname(cert, "sub.example.com", true) {
		t.Error("Failed to match subdomain (loose mode)")
	}
}
