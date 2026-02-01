package tlsutil

import (
	"crypto/x509"
	"strings"
)

// MatchHostname verifies that the certificate matches the given hostname.
// policy can be "strict" (string), true (bool, implies loose), or false (bool, skip).
func MatchHostname(cert *x509.Certificate, hostname string, policy interface{}) bool {
	// 1. Strict Check (Standard Library)
	err := cert.VerifyHostname(hostname)
	if err == nil {
		return true
	}

	// If policy is "strict", we fail if VerifyHostname failed
	if p, ok := policy.(string); ok && p == "strict" {
		return false
	}
    
    // If policy is boolean true, we allow loose check.
    // If policy is boolean false, the caller should have skipped this function, 
    // but if called, we assume it might mean "don't enforce strict", so we check loose?
    // Actually, if policy is false, we shouldn't be calling this? 
    // config.toml says: "false means no verification".
    // So the caller in proxy.go should check `if policy != false`.
    
    // We assume here we are in "loose" mode (policy == true or anything else that is not strict/false).

	return looselyMatch(cert, hostname)
}

func looselyMatch(cert *x509.Certificate, hostname string) bool {
    // Check SANs
    for _, dnsName := range cert.DNSNames {
        if isSameDomain(dnsName, hostname) {
            return true
        }
    }
    
    // Check CommonName
    if len(cert.DNSNames) == 0 && cert.Subject.CommonName != "" {
         if isSameDomain(cert.Subject.CommonName, hostname) {
            return true
        }
    }

	return false
}

// isSameDomain checks if two domains share the same registered domain (effectively last 2 parts)
// Replicates Python: dn.rsplit('.', maxsplit=2)[-2:] == hostname.rsplit('.', maxsplit=2)[-2:]
func isSameDomain(d1, d2 string) bool {
    p1 := getDomainParts(d1)
    p2 := getDomainParts(d2)
    
    if len(p1) != len(p2) {
        return false
    }
    for i := range p1 {
        if p1[i] != p2[i] {
            return false
        }
    }
    return true
}

func getDomainParts(d string) []string {
    parts := strings.Split(d, ".")
    if len(parts) > 2 {
        return parts[len(parts)-2:]
    }
    return parts
}
