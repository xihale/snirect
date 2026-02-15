package dns

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	miekgdns "github.com/miekg/dns"
	"snirect/internal/config"
)

// mockBackend implements dnsBackend for testing.
type mockBackend struct {
	aaaaResp  *miekgdns.Msg
	aResp     *miekgdns.Msg
	callCount int
}

func (m *mockBackend) Exchange(q *miekgdns.Msg) (*miekgdns.Msg, string, error) {
	m.callCount++
	qt := q.Question[0].Qtype
	switch qt {
	case miekgdns.TypeAAAA:
		if m.aaaaResp != nil {
			return m.aaaaResp, "::1", nil
		}
		return nil, "", fmt.Errorf("no AAAA response configured")
	case miekgdns.TypeA:
		if m.aResp != nil {
			return m.aResp, "127.0.0.1", nil
		}
		return nil, "", fmt.Errorf("no A response configured")
	default:
		return nil, "", fmt.Errorf("unexpected qtype %d", qt)
	}
}

// makeDNSResponse creates a DNS message with an answer section containing A or AAAA records.
func makeDNSResponse(qType uint16, ips []string, ttl uint32) *miekgdns.Msg {
	msg := new(miekgdns.Msg)
	msg.Rcode = miekgdns.RcodeSuccess
	msg.RecursionAvailable = true

	for _, ipStr := range ips {
		if qType == miekgdns.TypeA {
			ip := net.ParseIP(ipStr).To4()
			if ip == nil {
				continue
			}
			rr := &miekgdns.A{
				Hdr: miekgdns.RR_Header{Name: miekgdns.Fqdn("example.com"), Rrtype: miekgdns.TypeA, Class: miekgdns.ClassINET, Ttl: ttl},
				A:   ip,
			}
			msg.Answer = append(msg.Answer, rr)
		} else if qType == miekgdns.TypeAAAA {
			ip := net.ParseIP(ipStr)
			if ip == nil || ip.To4() != nil {
				continue
			}
			rr := &miekgdns.AAAA{
				Hdr:  miekgdns.RR_Header{Name: miekgdns.Fqdn("example.com"), Rrtype: miekgdns.TypeAAAA, Class: miekgdns.ClassINET, Ttl: ttl},
				AAAA: ip,
			}
			msg.Answer = append(msg.Answer, rr)
		}
	}
	return msg
}

func TestPreferenceCacheBasic(t *testing.T) {
	cache := newPreferenceCache(0)
	cache.set("test", "1.2.3.4", 0, 5*time.Second)
	ip, ok := cache.get("test")
	if !ok || ip != "1.2.3.4" {
		t.Fatalf("expected cached IP, got %v, ok=%v", ip, ok)
	}
	time.Sleep(6 * time.Second)
	ip, ok = cache.get("test")
	if ok {
		t.Fatalf("expected expired entry to be gone")
	}
}

func TestPreferenceCacheEviction(t *testing.T) {
	cache := newPreferenceCache(2)
	cache.set("a", "1.1.1.1", 0, time.Minute)
	cache.set("b", "2.2.2.2", 0, time.Minute)
	cache.set("c", "3.3.3.3", 0, time.Minute) // evicts one
	size, _, _ := cache.stats()
	if size != 2 {
		t.Fatalf("expected size 2, got %d", size)
	}
}

func TestPreferenceCacheConcurrent(t *testing.T) {
	cache := newPreferenceCache(100)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			host := fmt.Sprintf("%c.test.com", 'a'+rune(idx%26))
			cache.set(host, "127.0.0.1", 0, time.Second)
		}()
	}
	wg.Wait()
}

func TestResolver_StandardSelectsIPv6(t *testing.T) {
	aaaaMsg := makeDNSResponse(miekgdns.TypeAAAA, []string{"2001:db8::1"}, 300)
	aMsg := makeDNSResponse(miekgdns.TypeA, []string{"192.0.2.1"}, 300)
	backend := &mockBackend{aaaaResp: aaaaMsg, aResp: aMsg}

	r := &Resolver{
		Config: &config.Config{
			IPv6:       true,
			Preference: config.PreferenceConfig{},
			DNS:        config.DNSConfig{},
			Timeout:    config.TimeoutConfig{DNS: 5},
		},
		backend:   backend,
		cache:     make(map[string]cacheEntry),
		prefCache: newPreferenceCache(100),
	}

	ip, _, err := r.resolveStandard(context.Background(), "example.com", nil)
	if err != nil {
		t.Fatalf("resolveStandard error: %v", err)
	}
	if ip != "2001:db8::1" {
		t.Fatalf("expected IPv6 %s, got %s", "2001:db8::1", ip)
	}
}

func TestResolver_StandardFallbackToIPv4(t *testing.T) {
	emptyAAAA := makeDNSResponse(miekgdns.TypeAAAA, []string{}, 300)
	aMsg := makeDNSResponse(miekgdns.TypeA, []string{"192.0.2.1"}, 300)
	backend := &mockBackend{aaaaResp: emptyAAAA, aResp: aMsg}

	r := &Resolver{
		Config: &config.Config{
			IPv6:       true,
			Preference: config.PreferenceConfig{},
			DNS:        config.DNSConfig{},
			Timeout:    config.TimeoutConfig{DNS: 5},
		},
		backend:   backend,
		cache:     make(map[string]cacheEntry),
		prefCache: newPreferenceCache(100),
	}

	ip, _, err := r.resolveStandard(context.Background(), "example.com", nil)
	if err != nil {
		t.Fatalf("resolveStandard error: %v", err)
	}
	if ip != "192.0.2.1" {
		t.Fatalf("expected IPv4 %s, got %s", "192.0.2.1", ip)
	}
}

func TestResolver_IPv4Mode(t *testing.T) {
	aMsg := makeDNSResponse(miekgdns.TypeA, []string{"192.0.2.1"}, 300)
	backend := &mockBackend{aResp: aMsg}

	r := &Resolver{
		Config: &config.Config{
			IPv6:       true, // IPv6 enabled, but mode forces IPv4
			Preference: config.PreferenceConfig{Mode: config.IPPreferenceIPv4},
			DNS:        config.DNSConfig{},
			Timeout:    config.TimeoutConfig{DNS: 5},
		},
		backend:   backend,
		cache:     make(map[string]cacheEntry),
		prefCache: newPreferenceCache(100),
	}

	ip, err := r.resolveWithPreference(context.Background(), "example.com", nil)
	if err != nil {
		t.Fatalf("resolveWithPreference error: %v", err)
	}
	if ip != "192.0.2.1" {
		t.Fatalf("expected IPv4 %s, got %s", "192.0.2.1", ip)
	}
	// Ensure only A query was made (callCount should be 1)
	if backend.callCount != 1 {
		t.Fatalf("expected 1 DNS query, got %d", backend.callCount)
	}
}

func TestResolver_FastestFallbackWhenNoIPs(t *testing.T) {
	emptyMsg := makeDNSResponse(miekgdns.TypeA, []string{}, 300)
	backend := &mockBackend{aaaaResp: emptyMsg, aResp: emptyMsg}

	r := &Resolver{
		Config: &config.Config{
			IPv6: true,
			Preference: config.PreferenceConfig{
				Mode:          config.IPPreferenceFastest,
				TestTimeoutMs: 1,
				MaxTestIPs:    10,
				CacheTTL:      60,
				EnableTesting: true,
				TestParallel:  true,
			},
			DNS:     config.DNSConfig{},
			Timeout: config.TimeoutConfig{DNS: 5},
		},
		backend:   backend,
		cache:     make(map[string]cacheEntry),
		prefCache: newPreferenceCache(100),
	}

	ip, err := r.resolveFastest(context.Background(), "example.com", nil)
	// Since no IPs, resolveStandard will be called and fail (no records)
	if err == nil {
		t.Fatalf("expected error from fastest fallback, got ip %s", ip)
	}
}

func TestResolver_PreferenceCaching(t *testing.T) {
	aMsg := makeDNSResponse(miekgdns.TypeA, []string{"192.0.2.1"}, 300)
	backend := &mockBackend{aResp: aMsg}
	r := &Resolver{
		Config: &config.Config{
			IPv6: false,
			Preference: config.PreferenceConfig{
				CacheTTL: 60,
			},
			DNS:     config.DNSConfig{},
			Timeout: config.TimeoutConfig{DNS: 5},
		},
		backend:   backend,
		cache:     make(map[string]cacheEntry),
		prefCache: newPreferenceCache(100),
	}

	// First resolve should cache preference
	ip1, err := r.resolveWithPreference(context.Background(), "example.com", nil)
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	if ip1 != "192.0.2.1" {
		t.Fatalf("unexpected ip: %s", ip1)
	}
	// Second resolve should hit preference cache and not call backend
	backend.callCount = 0
	ip2, err := r.resolveWithPreference(context.Background(), "example.com", nil)
	if err != nil {
		t.Fatalf("second resolve error: %v", err)
	}
	if ip2 != ip1 {
		t.Fatalf("ip mismatch: %s vs %s", ip1, ip2)
	}
	if backend.callCount > 0 {
		t.Fatalf("backend was called after pref cache hit, calls: %d", backend.callCount)
	}
}

func TestResolver_lookupAllAddresses(t *testing.T) {
	aMsg := makeDNSResponse(miekgdns.TypeA, []string{"192.0.2.1", "192.0.2.2"}, 300)
	backend := &mockBackend{aResp: aMsg}
	r := &Resolver{
		Config:    &config.Config{},
		backend:   backend,
		cache:     make(map[string]cacheEntry),
		prefCache: newPreferenceCache(0),
	}
	records, err := r.lookupAllAddresses(context.Background(), "example.com", miekgdns.TypeA, nil)
	if err != nil {
		t.Fatalf("lookupAllAddresses error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	// Verify IPs present (order may not be guaranteed but both should be there)
	found1, found2 := false, false
	for _, r := range records {
		if r.ip == "192.0.2.1" {
			found1 = true
		}
		if r.ip == "192.0.2.2" {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Fatalf("expected both IPs in records")
	}
}

func BenchmarkPreferenceCacheSetGet(b *testing.B) {
	cache := newPreferenceCache(0)
	for i := 0; i < b.N; i++ {
		host := fmt.Sprintf("host%d.test.com", i)
		cache.set(host, "127.0.0.1", 0, time.Minute)
		_, _ = cache.get(host)
	}
}
