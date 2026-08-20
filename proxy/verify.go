package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"
	"time"

	"github.com/xihale/snirect/config"
	"github.com/xihale/snirect/logger"
	"github.com/xihale/snirect/rules"

	"golang.org/x/net/publicsuffix"
)

// TLSConnection is the minimal interface needed for certificate verification.
type TLSConnection interface {
	ConnectionState() tls.ConnectionState
}

// MatchHostname verifies that the certificate matches the given hostname.
func MatchHostname(cert *x509.Certificate, hostname string, policy rules.CertPolicy) bool {
	if strings.ContainsAny(hostname, "*?$") {
		for _, dnsName := range cert.DNSNames {
			if rules.MatchHost(hostname, dnsName) {
				return true
			}
		}
		if len(cert.DNSNames) == 0 && cert.Subject.CommonName != "" {
			if rules.MatchHost(hostname, cert.Subject.CommonName) {
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
		logger.Upstream().Debug("failed to get eTLD+1 for host", "host", hostname, "error", err)
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
// ignoreExpiry skips certificate time validity checks when true.
func VerifyCert(conn TLSConnection, host, targetSNI string, policy rules.CertPolicy, sec config.SecurityConfig, ignoreExpiry bool) bool {
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		logger.Upstream().Warn("upstream sent no certificates", "host", host)
		return false
	}
	leaf := state.PeerCertificates[0]

	// 1. Policy disabled: skip all verification. This is an expected, configured
	// state (e.g. bypass mode) on every intercepted host, so DEBUG not WARN to
	// avoid per-connection spam.
	if !policy.Enabled {
		logger.Upstream().Debug("certificate verification disabled by policy", "host", host)
		return true
	}

	// 2. Time validity check
	if sec.CheckValidity && !ignoreExpiry {
		now := time.Now()
		if now.Before(leaf.NotBefore) {
			logger.Upstream().Warn("certificate not valid yet", "host", host, "not_before", leaf.NotBefore.Format(time.RFC3339))
			return false
		}
		if now.After(leaf.NotAfter) {
			logger.Upstream().Warn("certificate expired", "host", host, "not_after", leaf.NotAfter.Format(time.RFC3339))
			return false
		}
	}

	// 3. EKU check
	if sec.CheckEKU && !hasServerAuthEKU(leaf) {
		logger.Upstream().Warn("certificate missing serverAuth EKU", "host", host)
		return false
	}

	// 4. Allowed list (highest priority)
	// Check if any certificate DNS name matches an allowed pattern.
	// Skips chain validation and time validity (transit proxy certs may be expired).
	if len(policy.Allowed) > 0 {
		allowedStrict := sec.AllowedStrict
		for _, allowedPattern := range policy.Allowed {
			for _, dnsName := range leaf.DNSNames {
				if allowedStrict {
					if dnsName == allowedPattern {
						logger.Upstream().Debug("certificate allowed by pattern", "host", host, "pattern", allowedPattern, "dns_name", dnsName)
						return true
					}
				} else {
					if rules.MatchHost(allowedPattern, dnsName) {
						logger.Upstream().Debug("certificate allowed by pattern", "host", host, "pattern", allowedPattern, "dns_name", dnsName)
						return true
					}
				}
			}
		}
		// Also check CommonName if no SANs
		if len(leaf.DNSNames) == 0 && leaf.Subject.CommonName != "" {
			for _, allowedPattern := range policy.Allowed {
				if allowedStrict {
					if leaf.Subject.CommonName == allowedPattern {
						return true
					}
				} else {
					if rules.MatchHost(allowedPattern, leaf.Subject.CommonName) {
						return true
					}
				}
			}
		}
		logger.Upstream().Debug("certificate domains did not match allowed list", "host", host, "domains", leaf.DNSNames, "allowed", policy.Allowed)
		return false
	}

	// 5. Certificate chain validation (only for standard hostname verification)
	if sec.ValidateChain {
		if err := verifyCertificateChain(leaf, state, sec, nil); err != nil {
			logger.Upstream().Warn("certificate chain validation failed", "host", host, "error", err)
			return false
		}
	}

	// 6. Standard hostname verification (original host)
	if MatchHostname(leaf, host, policy) {
		return true
	}

	// 7. If SNI was altered, also check against altered SNI
	if targetSNI != "" && targetSNI != host {
		if MatchHostname(leaf, targetSNI, policy) {
			logger.Upstream().Debug("verified certificate against altered SNI", "host", host, "target_sni", targetSNI)
			return true
		}
	}

	logger.Upstream().Warn("hostname does not match certificate domains", "host", host, "target_sni", targetSNI, "domains", leaf.DNSNames)
	return false
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
