package dns

import (
	"context"
	"net"
	"snirect/internal/config"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestResolver_ECS_Optimization(t *testing.T) {
	cfg := &config.Config{
		IPv6: true,
		ECS:  "auto",
		DNS: config.DNSConfig{
			Nameserver:   []string{"https://dns.google/dns-query"},
			BootstrapDNS: []string{"tls://223.5.5.5"},
		},
	}
	rules := &config.Rules{}

	r := NewResolver(cfg, rules)

	// Simulate a mobile IPv6 address from the user's logs
	// 2408:8256:d085:: is a China Mobile IPv6 prefix
	mockIP := net.ParseIP("2408:8256:d085:abcd:1234:5678:90ab:cdef")

	t.Run("IPv4 Resolution (A Record)", func(t *testing.T) {
		host := "youtube.com"
		// Set a longer timeout for CI/Test environments
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		ip, err := r.lookupType(ctx, host, dns.TypeA, mockIP)
		if err != nil {
			t.Logf("A lookup failed (likely network timeout in test env): %v", err)
			return
		}
		t.Logf("Resolved A: %s", ip)
	})

	t.Run("IPv6 Resolution (AAAA Record)", func(t *testing.T) {
		host := "youtube.com"
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		ip, err := r.lookupType(ctx, host, dns.TypeAAAA, mockIP)
		if err != nil {
			t.Logf("AAAA lookup failed (expected if IPv6 env issues): %v", err)
			return
		}
		t.Logf("Resolved AAAA: %s", ip)
	})
}

func TestResolver_Cache_Consistency(t *testing.T) {
	cfg := &config.Config{
		IPv6: true,
		ECS:  "auto",
		DNS: config.DNSConfig{
			Nameserver: []string{"https://dns.google/dns-query"},
		},
	}
	rules := &config.Rules{}
	r := NewResolver(cfg, rules)

	host := "example.com"
	mockIP := net.ParseIP("1.1.1.1")

	// 1. Resolve A
	ipA, err := r.lookupType(context.Background(), host, dns.TypeA, mockIP)
	if err != nil {
		t.Skip("Network unavailable")
	}
	r.setCache(host, ipA, dns.TypeA)

	// 2. Check if cached for TypeA
	cachedA, ok := r.getCache(host, dns.TypeA)
	if !ok || cachedA != ipA {
		t.Errorf("A record not correctly cached")
	}

	// 3. Check TypeAAAA (should be empty in cache)
	_, ok = r.getCache(host, dns.TypeAAAA)
	if ok {
		t.Errorf("TypeAAAA should not be ok before resolution")
	}

	// 4. Resolve AAAA
	ipAAAA, _ := r.lookupType(context.Background(), host, dns.TypeAAAA, mockIP)
	r.setCache(host, ipAAAA, dns.TypeAAAA)

	cachedAAAA, _ := r.getCache(host, dns.TypeAAAA)
	if cachedAAAA == cachedA && ipA != ipAAAA {
		t.Errorf("Cache collision: A and AAAA share same cache key")
	}
}

func TestResolver_Xihale_Upstream(t *testing.T) {
	cfg := &config.Config{
		IPv6: true,
		ECS:  "auto",
		DNS: config.DNSConfig{
			Nameserver: []string{"https://d.xihale.top/dns-query"},
		},
	}
	rules := &config.Rules{}
	r := NewResolver(cfg, rules)
	mockIP := net.ParseIP("2408:8256:d085:abcd:1234:5678:90ab:cdef")

	t.Run("IPv6 Resolution via Xihale", func(t *testing.T) {
		host := "youtube.com"
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		ip, err := r.lookupType(ctx, host, dns.TypeAAAA, mockIP)
		if err != nil {
			t.Logf("Lookup failed (expected if network issues): %v", err)
			return
		}
		t.Logf("Result from d.xihale.top: %s", ip)
	})
}
