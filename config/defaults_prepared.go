// Hand-maintained compile-time defaults. Keep commented examples in
// config.toml aligned with this struct.

package config

// PreparsedDefaultConfig contains the compile-time parsed default configuration.
var PreparsedDefaultConfig = Config{
	CheckHostname: true,
	SetProxy:      true,
	CAInstall:     "auto",
	IPv6:          false,
	DNS: DNSConfig{
		Nameserver:   []string{"https://dnschina1.soraharu.com/dns-query", "tls://223.5.5.5"},
		BootstrapDNS: []string{"tls://223.5.5.5"},
	},
	Timeout: TimeoutConfig{
		Dial: 30,
		DNS:  5,
	},
	Limit: LimitConfig{
		DNSCacheSize: 10000,
	},
	Log: LogConfig{
		Level: "INFO",
	},
	Server: ServerConfig{
		Address: "127.0.0.1",
		Port:    7654,
		PACHost: "127.0.0.1",
	},
	Preference: PreferenceConfig{
		Mode:          "standard",
		TestTimeoutMs: 500,
		MaxTestIPs:    10,
		CacheTTL:      300,
		CacheSize:     5000,
	},
	Security: SecurityConfig{
		ValidateChain:  true,
		MinChainLength: 2,
		CheckEKU:       true,
		CheckValidity:  true,
	},
}
