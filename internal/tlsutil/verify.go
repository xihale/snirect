package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"snirect/internal/config"
	"snirect/internal/logger"
	"strings"
	"time"

	"golang.org/x/net/publicsuffix"
)

// TLSConnection is the minimal interface needed for certificate verification.
type TLSConnection interface {
	ConnectionState() tls.ConnectionState
}

// MatchHostname verifies that the certificate matches the given hostname.
func MatchHostname(cert *x509.Certificate, hostname string, policy config.CertPolicy) bool {
	if strings.ContainsAny(hostname, "*?$") {
		for _, dnsName := range cert.DNSNames {
			if config.MatchPattern(hostname, dnsName) {
				return true
			}
		}
		if len(cert.DNSNames) == 0 && cert.Subject.CommonName != "" {
			if config.MatchPattern(hostname, cert.Subject.CommonName) {
				return true
			}
		}
	}

	// 1. Strict Check (Standard Library)
	err := cert.VerifyHostname(hostname)
	if err == nil {
		return true
	}

	// If policy is strict, we fail if VerifyHostname failed
	if policy.Strict {
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

// VerifyCert verifies a TLS connection's certificate against the given host, policy, and security config.
// This is the shared verification logic used by both proxy and upstream client.
func VerifyCert(conn TLSConnection, host, targetSNI string, policy config.CertPolicy, sec config.SecurityConfig) bool {
	// Global override to skip all verification (development/debugging only)
	if skip := os.Getenv("TLS_INSECURE_SKIP_VERIFY"); skip == "true" || skip == "1" {
		logger.Debug("TLS verification skipped due to TLS_INSECURE_SKIP_VERIFY env var")
		return true
	}

	// Fast path for skipping verification
	if !policy.Enabled {
		return true
	}

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		logger.Warn("no peer certificates presented")
		return false
	}
	leaf := state.PeerCertificates[0]

	// 1. Time validity check (always perform)
	if sec.CheckValidity {
		now := time.Now()
		if now.Before(leaf.NotBefore) {
			logger.Warn("certificate not valid yet: %s", leaf.NotBefore.Format(time.RFC3339))
			return false
		}
		if now.After(leaf.NotAfter) {
			logger.Warn("certificate expired: %s", leaf.NotAfter.Format(time.RFC3339))
			return false
		}
	}

	// 2. Certificate chain validation
	if sec.ValidateChain {
		if err := verifyCertificateChain(leaf, state, sec, nil); err != nil {
			logger.Warn("chain validation: %v", err)
			return false
		}
	}

	// 3. EKU check
	if sec.CheckEKU && !hasServerAuthEKU(leaf) {
		logger.Warn("certificate missing serverAuth EKU")
		return false
	}

	// 4. Allowed list (highest priority)
	if len(policy.Allowed) > 0 {
		allowedStrict := sec.AllowedStrict
		for _, pattern := range policy.Allowed {
			// If allowed_strict is true, we must not allow wildcards in matching
			// By using strict hostname matching (no wildcard expansion in pattern)
			matched := matchAllowedPattern(pattern, host, allowedStrict)
			if matched {
				logger.Debug("cert allowed by pattern %s matching %s", pattern, host)
				return true
			}
		}
		logger.Debug("cert domains %v did not match allowed list %v", leaf.DNSNames, policy.Allowed)
		return false
	}

	// 5. Standard hostname verification (original host)
	if MatchHostname(leaf, host, policy) {
		return true
	}

	// 6. If SNI was altered, also check against altered SNI
	if targetSNI != "" && targetSNI != host {
		if MatchHostname(leaf, targetSNI, policy) {
			logger.Debug("verified cert against altered SNI: %s", targetSNI)
			return true
		}
	}

	logger.Debug("hostname %s (SNI: %s) does not match cert domains %v", host, targetSNI, leaf.DNSNames)
	return false
}

// matchAllowedPattern matches host against pattern with optional strict mode.
// In strict mode, wildcards in pattern are not expanded (exact match only).
func matchAllowedPattern(pattern, host string, strict bool) bool {
	if strict {
		// Strict mode: pattern must exactly match host (no wildcard)
		return pattern == host
	}
	// Default: allow pattern matching (wildcards)
	return MatchHostnameByPattern(pattern, host)
}

// MatchHostnameByPattern uses pattern matching (wildcards) to check if host matches pattern.
func MatchHostnameByPattern(pattern, host string) bool {
	// Use the existing MatchPattern but ensure pattern can contain wildcards
	return config.MatchPattern(pattern, host)
}

// hasServerAuthEKU checks if the certificate has serverAuth extended key usage.
func hasServerAuthEKU(cert *x509.Certificate) bool {
	for _, eku := range cert.ExtKeyUsage {
		if eku == x509.ExtKeyUsageServerAuth {
			return true
		}
	}
	return false
}

// verifyCertificateChain validates the certificate chain up to a trusted root.
// If roots is nil, system root CAs are used.
func verifyCertificateChain(leaf *x509.Certificate, state tls.ConnectionState, sec config.SecurityConfig, roots *x509.CertPool) error {
	// Build intermediate pool from peer certificates (excluding leaf)
	intermediates := x509.NewCertPool()
	for _, cert := range state.PeerCertificates[1:] {
		intermediates.AddCert(cert)
	}

	// Use provided roots or fall back to system roots
	if roots == nil {
		var err error
		roots, err = x509.SystemCertPool()
		if err != nil {
			return fmt.Errorf("failed to get system cert pool: %w", err)
		}
		if roots == nil {
			return fmt.Errorf("system cert pool is nil")
		}
	}

	// Build verification options
	opts := x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		CurrentTime:   time.Now(),
	}

	// Verify the chain
	_, err := leaf.Verify(opts)
	if err != nil {
		return fmt.Errorf("chain verification failed: %w", err)
	}

	// Check chain length
	chain := state.PeerCertificates
	if len(chain) < sec.MinChainLength {
		return fmt.Errorf("chain too short: %d < %d", len(chain), sec.MinChainLength)
	}

	return nil
}
