package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	IPPreferenceStandard IPPreferenceMode = "standard" // Current behavior: prefer v6 if enabled, then first
	IPPreferenceFastest  IPPreferenceMode = "fastest"  // Test both, use lowest latency (cached)
	IPPreferenceIPv6     IPPreferenceMode = "ipv6"     // Always prefer IPv6 if available
	IPPreferenceIPv4     IPPreferenceMode = "ipv4"     // Always prefer IPv4
)

// PreferenceConfig contains DNS IP preference settings.
type PreferenceConfig struct {
	Mode          IPPreferenceMode `toml:"mode"`           // Selection mode
	EnableTesting bool             `toml:"enable_testing"` // Whether to do latency testing (for fastest mode)
	// TestTimeoutMs is the timeout for each connection test in milliseconds.
	// When testing IPs, we dial with this timeout and measure connection establishment time.
	TestTimeoutMs int `toml:"test_timeout_ms"`
	// TestParallel determines whether to test all IPs in parallel (true) or sequentially (false).
	TestParallel bool `toml:"test_parallel"`
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

// GetDefaultLogPath returns the platform-specific default log file path.
func GetDefaultLogPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "snirect.log" // Fallback to current directory
	}

	switch runtime.GOOS {
	case "linux":
		// XDG Base Directory: ~/.local/state/snirect/snirect.log
		// Or ~/.cache/snirect/snirect.log if state is not preferred by some distros, but state is better for logs.
		// Let's stick to XDG_STATE_HOME or ~/.local/state
		stateHome := os.Getenv("XDG_STATE_HOME")
		if stateHome == "" {
			stateHome = filepath.Join(homeDir, ".local", "state")
		}
		return filepath.Join(stateHome, "snirect", "snirect.log")
	case "darwin":
		// ~/Library/Logs/snirect/snirect.log
		return filepath.Join(homeDir, "Library", "Logs", "snirect", "snirect.log")
	case "windows":
		// %LOCALAPPDATA%\snirect\Logs\snirect.log
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(homeDir, "AppData", "Local")
		}
		return filepath.Join(localAppData, "snirect", "Logs", "snirect.log")
	default:
		return "snirect.log"
	}
}
