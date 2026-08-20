// Package config defines configuration structures and provides loading, validation, and default
// management for Snirect. It supports TOML configuration and compile-time default values.
package config

import (
	"github.com/xihale/snirect/rules"
)

// Config represents the main configuration for Snirect.
type Config struct {
	// CheckHostname controls certificate hostname verification.
	// Can be bool, string ("strict", "true", "false"), or []string of allowed hostnames.
	// Use CheckHostnamePolicy() for type-safe access.
	CheckHostname interface{} `toml:"check_hostname"`

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

	// Security contains certificate verification hardening settings.
	Security SecurityConfig `toml:"security"`
}

// SecurityConfig controls zero-trust certificate verification.
type SecurityConfig struct {
	// ValidateChain enables full certificate chain validation (default true).
	ValidateChain bool `toml:"validate_chain"`
	// MinChainLength minimum expected chain length including root (2 = self-signed, 3 = intermediate).
	MinChainLength int `toml:"min_chain_length"`
	// CheckEKU verifies certificate Extended Key Usage contains serverAuth (default true).
	CheckEKU bool `toml:"check_eku"`
	// CheckValidity verifies certificate time validity (always recommended, default true).
	CheckValidity bool `toml:"check_validity"`
	// AllowedStrict disables wildcards in allowed lists when true (default false).
	AllowedStrict bool `toml:"allowed_strict"`
}

// IPPreferenceMode defines how IP selection works when both IPv6 and IPv4 are available.
type IPPreferenceMode string

const (
	IPPreferenceStandard IPPreferenceMode = "standard" // Current behavior: prefer v6 if enabled, then first
	IPPreferenceFastest  IPPreferenceMode = "fastest"  // Test both, use lowest latency (cached)
	IPPreferenceIPv6     IPPreferenceMode = "ipv6"     // Always prefer IPv6 if available
	IPPreferenceIPv4     IPPreferenceMode = "ipv4"     // Always prefer IPv4
)

// PreferenceConfig contains DNS IP preference settings.
type PreferenceConfig struct {
	Mode IPPreferenceMode `toml:"mode"` // Selection mode
	// TestTimeoutMs is the timeout for each connection test in milliseconds.
	// When testing IPs, we dial with this timeout and measure connection establishment time.
	TestTimeoutMs int `toml:"test_timeout_ms"`
	// MaxTestIPs limits the number of IPs to test per query to avoid resource exhaustion.
	MaxTestIPs int `toml:"max_test_ips"`
	// CacheTTL is the preference cache TTL in seconds. 0 means use DNS TTL / 2.
	CacheTTL int `toml:"cache_ttl"`
	// CacheSize limits the number of entries in the preference cache. 0 = unlimited.
	CacheSize int `toml:"cache_size"`
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

// CheckHostnamePolicy returns the parsed certificate verification policy from CheckHostname.
// This provides type-safe access instead of passing the raw interface{} to callers.
func (c *Config) CheckHostnamePolicy() rules.CertPolicy {
	p, _ := rules.ParseCertPolicy(c.CheckHostname)
	return p
}
