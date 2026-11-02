// Package config defines configuration structures and provides loading, validation, and default
// management for Snirect. It supports TOML configuration and compile-time default values.
package config

import (
	"github.com/xihale/snirect/internal/rules"
)

// Config represents the main configuration for Snirect.
type Config struct {
	// CheckHostname controls certificate hostname verification:
	// "off" (skip), "loose" (default; subdomains may match), or "strict"
	// (standard library matching only).
	CheckHostname string `toml:"check_hostname"`

	// CheckHostnameAllow, when non-empty, restricts verification to hosts
	// matching these patterns (certificates for transit proxies etc.).
	// Non-empty overrides the CheckHostname mode entirely.
	CheckHostnameAllow []string `toml:"check_hostname_allow"`

	SetProxy bool `toml:"set_proxy"`

	// CAInstall controls the root CA installation policy ("auto", "always", "never").
	CAInstall string `toml:"ca_install"`

	// IPv6 enables or disables IPv6 support.
	IPv6 bool `toml:"ipv6"`

	// ECS (EDNS Client Subnet) configuration ("auto", CIDR, or empty).
	ECS string `toml:"ecs"`

	// DNS contains upstream resolver settings.
	DNS DNSConfig `toml:"DNS"`

	// Timeout contains various timeout settings.
	Timeout TimeoutConfig `toml:"timeout"`

	// Limit contains resource usage limit settings.
	Limit LimitConfig `toml:"limit"`

	// Log contains logging configuration.
	Log LogConfig `toml:"log"`

	// Server contains local proxy server settings.
	Server ServerConfig `toml:"server"`

	// Preference contains DNS IP preference settings.
	Preference PreferenceConfig `toml:"preference"`
}

// IPPreferenceMode defines how IP selection works when both IPv6 and IPv4 are available.
type IPPreferenceMode string

const (
	IPPreferenceStandard IPPreferenceMode = "standard" // Prefer v6 if enabled, then first
	IPPreferenceIPv6     IPPreferenceMode = "ipv6"     // Always prefer IPv6 if available
	IPPreferenceIPv4     IPPreferenceMode = "ipv4"     // Always prefer IPv4
)

// PreferenceConfig contains DNS IP preference settings.
type PreferenceConfig struct {
	Mode IPPreferenceMode `toml:"mode"` // Selection mode
}

// TimeoutConfig contains timeout settings in seconds.
type TimeoutConfig struct {
	Dial int `toml:"dial"` // Dial timeout for remote connections
	DNS  int `toml:"dns"`  // DNS query timeout
}

// LimitConfig contains resource limit settings.
type LimitConfig struct {
	MaxConns     int `toml:"max_connections"` // Maximum concurrent connections (0 = unlimited)
	DNSCacheSize int `toml:"dns_cache_size"`  // Maximum DNS cache entries
}

// DNSConfig contains DNS resolver settings.
type DNSConfig struct {
	Nameserver   []string `toml:"nameserver"`    // Upstream DNS servers (UDP/TCP/TLS/DoH)
	BootstrapDNS []string `toml:"bootstrap_dns"` // DNS servers for bootstrapping encryption
}

// LogConfig contains logging settings.
type LogConfig struct {
	Level string `toml:"loglevel"` // Log level (DEBUG, INFO, WARN, ERROR)
	File  string `toml:"logfile"`  // Path to log file
}

// ServerConfig contains proxy server settings.
type ServerConfig struct {
	Address    string `toml:"address"`     // Bind address (e.g., "127.0.0.1")
	Port       int    `toml:"port"`        // Listen port
	PACHost    string `toml:"pac_host"`    // Hostname for PAC file generation
	BufferSize int    `toml:"buffer_size"` // Tunnel copy buffer size in bytes (default 65536, min 4096, max 1048576)
}

// CheckHostnamePolicy returns the certificate verification policy derived
// from CheckHostname/CheckHostnameAllow.
func (c *Config) CheckHostnamePolicy() rules.CertPolicy {
	if len(c.CheckHostnameAllow) > 0 {
		return rules.CertPolicy{Enabled: true, Allowed: c.CheckHostnameAllow}
	}
	switch c.CheckHostname {
	case "off":
		return rules.CertPolicy{Enabled: false}
	case "strict":
		return rules.CertPolicy{Enabled: true, Strict: true}
	default: // "loose", or the zero value of programmatically built configs
		return rules.CertPolicy{Enabled: true}
	}
}
