package dns

import (
	"testing"

	"github.com/xihale/snirect/internal/config"
	ruleslib "github.com/xihale/snirect/internal/rules"
)

func TestEnsureHostPort(t *testing.T) {
	cases := []struct {
		in, port, want string
	}{
		{"223.5.5.5", "853", "223.5.5.5:853"},
		{"223.5.5.5:853", "853", "223.5.5.5:853"},
		{"[2001:db8::1]", "853", "[2001:db8::1]:853"},
		{"[2001:db8::1]:853", "853", "[2001:db8::1]:853"},
		{"dns.google", "853", "dns.google:853"},
		{"dns.google:853", "853", "dns.google:853"},
	}
	for _, tc := range cases {
		got := ensureHostPort(tc.in, tc.port)
		if got != tc.want {
			t.Errorf("ensureHostPort(%q, %q) = %q, want %q", tc.in, tc.port, got, tc.want)
		}
	}
}

func TestIsHostnameEndpoint(t *testing.T) {
	hostnames := []string{
		"https://dnschina1.soraharu.com/dns-query",
		"https://dns.google/dns-query",
		"tls://dns.google",
		"tls://dns.google:853",
		"dns.google",
	}
	for _, ns := range hostnames {
		if !isHostnameEndpoint(ns) {
			t.Errorf("isHostnameEndpoint(%q) = false, want true", ns)
		}
	}
	literals := []string{
		"tls://223.5.5.5",
		"tls://223.5.5.5:853",
		"https://1.1.1.1/dns-query",
		"https://[2606:4700:4700::1111]/dns-query",
		"1.1.1.1",
		"1.1.1.1:53",
		"udp://8.8.8.8:53",
	}
	for _, ns := range literals {
		if isHostnameEndpoint(ns) {
			t.Errorf("isHostnameEndpoint(%q) = true, want false", ns)
		}
	}
}

func TestNewBackend_TlsDefaultPort(t *testing.T) {
	cfg := &config.Config{
		DNS:     config.DNSConfig{Nameserver: []string{"tls://223.5.5.5"}},
		Timeout: config.TimeoutConfig{DNS: 5},
	}
	b := newBackend(cfg, ruleslib.NewRules(), nil)
	std, ok := b.(*stdBackend)
	if !ok || len(std.upstreams) != 1 {
		t.Fatalf("expected 1 upstream, got %T %#v", b, b)
	}
	up, ok := std.upstreams[0].(*dnsUpstream)
	if !ok {
		t.Fatalf("expected *dnsUpstream, got %T", std.upstreams[0])
	}
	if up.addr != "223.5.5.5:853" {
		t.Errorf("tls default port: addr = %q, want 223.5.5.5:853", up.addr)
	}
}

func TestNewBackend_HostnameUpstreamWithBootstrap(t *testing.T) {
	cfg := &config.Config{
		DNS: config.DNSConfig{
			Nameserver:   []string{"https://dnschina1.soraharu.com/dns-query", "tls://223.5.5.5"},
			BootstrapDNS: []string{"tls://223.5.5.5"},
		},
		Timeout: config.TimeoutConfig{DNS: 5},
	}
	b := newBackend(cfg, ruleslib.NewRules(), nil)
	if b == nil {
		t.Fatal("expected backend when hostname DoH is paired with IP-literal bootstrap")
	}
	std := b.(*stdBackend)
	if len(std.upstreams) != 2 {
		t.Fatalf("expected 2 upstreams, got %d", len(std.upstreams))
	}
	doh, ok := std.upstreams[0].(*dohUpstream)
	if !ok {
		t.Fatalf("upstream 0: expected *dohUpstream, got %T", std.upstreams[0])
	}
	if doh.addr != "https://dnschina1.soraharu.com/dns-query" {
		t.Errorf("DoH URL rewritten: %q", doh.addr)
	}
}

func TestNewBootstrapResolver_SkipsHostname(t *testing.T) {
	// A hostname-only bootstrap list cannot resolve itself.
	resolve := newBootstrapResolver([]string{"https://dns.google/dns-query"}, 5e9, nil)
	if resolve != nil {
		t.Fatal("expected nil resolver when every bootstrap entry is a hostname")
	}
	resolve = newBootstrapResolver([]string{"tls://223.5.5.5"}, 5e9, nil)
	if resolve == nil {
		t.Fatal("expected resolver for IP-literal bootstrap")
	}
}
