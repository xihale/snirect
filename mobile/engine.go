package core

import (
	"fmt"
	"net"
	"strings"

	coreconfig "github.com/xihale/snirect/internal/config"
	coredns "github.com/xihale/snirect/internal/dns"
	ruleslib "github.com/xihale/snirect/internal/rules"
)

// Config is the JSON-shaped engine configuration handed in from the Kotlin side
// (SnirectEngineConfig → CoreConfig). It is mobile-specific: the desktop
// config.Config is richer (TOML, security/server sections) and not all of it is
// reachable from Android, so we keep this slim mirror and expand it into a
// core config.Config where the shared resolver/proxy code needs one.
type Config struct {
	NameServers  []string `json:"nameservers"`
	BootstrapDNS []string `json:"bootstrap_dns"`
	CheckHN      bool     `json:"check_hostname"`
	MTU          int      `json:"mtu"`
	EnableIPv6   bool     `json:"enable_ipv6"`
	LogLevel     string   `json:"log_level"`
}

// initEngine loads the compiled built-in rules and builds the resolver from an
// already-parsed mobile Config. It returns the expanded core config (for the
// proxy), the resolver, the rules, and the protect() bypass dialer — the dialer
// is shared by the resolver (DNS upstreams escape the TUN) and injected into
// the proxy (upstream TCP dials escape the TUN). Returns an error if the DNS
// setup cannot work on Android or rules fail to load.
func initEngine(config *Config, cb EngineCallbacks) (*coreconfig.Config, *coredns.Resolver, *ruleslib.Rules, *net.Dialer, error) {
	// DNS validation (audit B2). Android has no /etc/resolv.conf, so a Go
	// resolver with an empty upstream list silently produces a nil backend —
	// every query would then be dropped client-side with no error anywhere.
	// An empty nameserver list is therefore a hard startup failure.
	if len(config.NameServers) == 0 {
		return nil, nil, nil, nil, fmt.Errorf("no DNS nameservers configured")
	}
	// Hostname DoH/DoT needs bootstrap DNS: Android has no /etc/resolv.conf,
	// so the Go system resolver hits 127.0.0.1:53 (nothing listens there).
	// The core resolver now resolves those hostnames via BootstrapDNS
	// (IP-literal only). Warn only when a hostname upstream is configured
	// without a usable bootstrap — otherwise the warning was a lie.
	hasHostname := false
	for _, ns := range config.NameServers {
		if isHostnameUpstream(ns) {
			hasHostname = true
			break
		}
	}
	if hasHostname && !hasIPLiteralBootstrap(config.BootstrapDNS) {
		LogWarn("Engine: hostname DNS upstreams need an IP-literal bootstrap (bootstrap_dns); they cannot resolve via the Android system resolver")
	} else if hasHostname {
		LogInfo("Engine: hostname DNS upstreams will be resolved via bootstrap %s", strings.Join(config.BootstrapDNS, ", "))
	}

	rules, err := ruleslib.LoadRules()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to load built-in rules: %v", err)
	}

	coreCfg := buildCoreConfig(config)

	// protect() bypass dialer: the single source for all outbound sockets that
	// must escape the VPN's own TUN. The resolver's upstream DNS queries and the
	// proxy's upstream TCP dials both go through it.
	bypass := bypassDialer(cb)
	resolver := coredns.NewResolver(coreCfg, rules, coredns.WithDialer(bypass))

	// NewResolver eagerly builds its backend; a nil backend means every
	// upstream failed to parse (the empty case was rejected above). Without
	// this check the engine would run with silently dead DNS (audit B2).
	if resolver.Backend() == nil {
		return nil, nil, nil, nil, fmt.Errorf("no usable DNS upstreams: all %d nameservers failed to parse", len(config.NameServers))
	}

	LogInfo("Engine: Initialized with built-in rules")
	return coreCfg, resolver, rules, bypass, nil
}

// isHostnameUpstream reports whether a nameserver entry is an http(s)/tls
// upstream whose host is a name rather than an IP literal. Plain UDP/TCP
// entries default to IP literals in practice and are not warned about here.
func isHostnameUpstream(ns string) bool {
	lower := strings.ToLower(ns)
	var rest string
	switch {
	case strings.HasPrefix(lower, "https://"):
		rest = ns[len("https://"):]
	case strings.HasPrefix(lower, "http://"):
		rest = ns[len("http://"):]
	case strings.HasPrefix(lower, "tls://"):
		rest = ns[len("tls://"):]
	default:
		return false
	}
	if i := strings.IndexAny(rest, "/"); i >= 0 {
		rest = rest[:i] // strip the DoH path
	}
	if host, _, err := net.SplitHostPort(rest); err == nil {
		rest = host // strip :853 etc.
	} else {
		// A bracketed IPv6 literal without a port ("https://[::1]/dns-query")
		// fails SplitHostPort; trim the brackets so it is not miswarned.
		rest = strings.Trim(rest, "[]")
	}
	return net.ParseIP(strings.TrimSpace(rest)) == nil
}

func hasIPLiteralBootstrap(servers []string) bool {
	for _, ns := range servers {
		if strings.TrimSpace(ns) != "" && !isHostnameUpstream(ns) {
			return true
		}
	}
	return false
}

// buildCoreConfig expands the slim mobile Config into a core config.Config so
// the shared resolver/proxy code (which reads the richer struct) gets sensible
// values. Mobile exposes only the knobs below; everything else stays at zero.
//
// Server.Port=0 asks the proxy to auto-pick a free loopback port; Server.Address
// is 127.0.0.1 so the netstack bridge's loopback hop never touches the TUN.
func buildCoreConfig(c *Config) *coreconfig.Config {
	return &coreconfig.Config{
		IPv6:          c.EnableIPv6,
		CheckHostname: c.CheckHN,
		DNS: coreconfig.DNSConfig{
			Nameserver:   c.NameServers,
			BootstrapDNS: c.BootstrapDNS,
		},
		Timeout: coreconfig.TimeoutConfig{
			Dial: 30,
			DNS:  5,
		},
		Limit: coreconfig.LimitConfig{
			DNSCacheSize: 10000,
		},
		Preference: coreconfig.PreferenceConfig{
			Mode: coreconfig.IPPreferenceStandard,
		},
		Server: coreconfig.ServerConfig{
			Address: "127.0.0.1",
			Port:    0, // auto-pick
		},
	}
}
