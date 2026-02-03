package tlsutil

import (
	"crypto/x509"

	"golang.org/x/net/publicsuffix"
	"snirect/internal/logger"
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

	return looselyMatch(cert, hostname)
}

func looselyMatch(cert *x509.Certificate, hostname string) bool {
	hostETLD1, err := publicsuffix.EffectiveTLDPlusOne(hostname)
	if err != nil {
		logger.Debug("Failed to get eTLD+1 for host %s: %v", hostname, err)
		return false
	}

	// Check SANs
	for _, dnsName := range cert.DNSNames {
		certETLD1, err := publicsuffix.EffectiveTLDPlusOne(dnsName)
		if err != nil {
			continue
		}
		if hostETLD1 == certETLD1 {
			return true
		}
	}

	// Check CommonName
	if len(cert.DNSNames) == 0 && cert.Subject.CommonName != "" {
		certETLD1, err := publicsuffix.EffectiveTLDPlusOne(cert.Subject.CommonName)
		if err == nil && hostETLD1 == certETLD1 {
			return true
		}
	}

	return false
}
