package config

import (
	"fmt"
)

// CertPolicy represents a certificate verification policy.
// It can be a boolean (true/false), a string ("strict", "false"), or a list of allowed hostnames.
type CertPolicy struct {
	Enabled bool
	Strict  bool
	Allowed []string
}

// ParseCertPolicy converts a raw configuration value into a CertPolicy.
func ParseCertPolicy(data interface{}) (CertPolicy, error) {
	var p CertPolicy
	switch v := data.(type) {
	case bool:
		p.Enabled = v
	case string:
		switch v {
		case "strict":
			p.Enabled = true
			p.Strict = true
		case "false":
			p.Enabled = false
		case "true":
			p.Enabled = true
		default:
			p.Enabled = true
			p.Allowed = []string{v}
		}
	case []interface{}:
		p.Enabled = true
		for _, item := range v {
			if s, ok := item.(string); ok {
				p.Allowed = append(p.Allowed, s)
			}
		}
	case nil:
		// Default zero value
	default:
		return p, fmt.Errorf("invalid cert policy type: %T", data)
	}
	return p, nil
}

// Config represents the main configuration for Snirect.
type Config struct {
	// CheckHostname controls certificate hostname verification.
	// Can be bool, string ("strict", "true", "false"), or []string of allowed hostnames.
	CheckHostname interface{} `toml:"check_hostname"`

	// SetProxy indicates whether to automatically set the system proxy.
	SetProxy bool `toml:"set_proxy"`

	// ImportCA controls the root CA installation policy ("auto", "always", "never").
	ImportCA string `toml:"importca"`

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
	Nameserver   []string `toml:"nameserver"`    // Upstream DNS servers (DoH/DoT/DoQ)
	BootstrapDNS []string `toml:"bootstrap_dns"` // DNS servers for bootstrapping encryption
}

// LogConfig contains logging settings.
type LogConfig struct {
	Level string `toml:"loglevel"` // Log level (DEBUG, INFO, WARN, ERROR)
	File  string `toml:"logfile"`  // Path to log file
}

// ServerConfig contains proxy server settings.
type ServerConfig struct {
	Address string `toml:"address"`  // Bind address (e.g., "127.0.0.1")
	Port    int    `toml:"port"`     // Listen port
	PACHost string `toml:"pac_host"` // Hostname for PAC file generation
}
