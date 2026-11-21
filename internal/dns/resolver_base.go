// Package dns provides multi-backend DNS resolution (UDP/TCP/TLS/DoH) with caching
// and IP preference selection. It is used by the proxy for reliable hostname resolution.
package dns

import (
	"context"
	"errors"
	"fmt"
	"github.com/xihale/snirect/internal/config"
	"github.com/xihale/snirect/internal/logger"
	"github.com/xihale/snirect/internal/rules"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/sync/singleflight"
)

type dnsBackend interface {
	Exchange(m *dns.Msg) (*dns.Msg, string, error)
}

type dnsContextBackend interface {
	ExchangeContext(ctx context.Context, m *dns.Msg) (*dns.Msg, string, error)
}

type dnsCacheKey struct {
	host  string
	qType uint16
}

type cacheEntry struct {
	ip           string
	negative     bool
	expiresAt    time.Time
	lastAccessed atomic.Int64 // UnixNano, updated lock-free under RLock
}

type ipRecord struct {
	ip  string
	ttl uint32
}

var errNegativeCacheHit = errors.New("dns negative cache hit")

type negativeDNSError struct {
	ttl time.Duration
	err error
}

func (e *negativeDNSError) Error() string { return e.err.Error() }
func (e *negativeDNSError) Unwrap() error { return e.err }

type Resolver struct {
	Config  *config.Config
	Rules   *rules.Rules
	backend dnsBackend

	cache   map[dnsCacheKey]*cacheEntry
	cacheMu sync.RWMutex

	lookupSF singleflight.Group
	systemSF singleflight.Group

	staticECSOnce sync.Once
	staticECSNet  *net.IPNet

	autoECSNet4  *net.IPNet
	autoECSNet6  *net.IPNet
	autoECSNetMu sync.RWMutex

	negativeCacheTTL time.Duration // Optional override for testing; defaults to 60s

	dialer   *net.Dialer // Optional custom dialer for upstream connections
	stopChan chan struct{}
}

// ResolverOption is a functional option for configuring a Resolver.
type ResolverOption func(*Resolver)

// WithDialer sets a custom net.Dialer for DNS upstream connections.
// This is useful for mobile platforms that need to protect socket file descriptors.
func WithDialer(d *net.Dialer) ResolverOption {
	return func(r *Resolver) {
		r.dialer = d
	}
}

// NewResolver creates a new Resolver instance based on the provided configuration.
func NewResolver(cfg *config.Config, rules *rules.Rules, opts ...ResolverOption) *Resolver {
	r := &Resolver{
		Config:   cfg,
		Rules:    rules,
		cache:    make(map[dnsCacheKey]*cacheEntry),
		stopChan: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(r)
	}

	r.initStaticECS()
	r.backend = newBackend(cfg, rules, r.dialer)

	go r.cleanCacheRoutine()

	if cfg.ECS == "auto" {
		go r.initAutoECS()
	}

	return r
}

func (r *Resolver) initAutoECS() {
	var wg sync.WaitGroup
	wg.Add(2)

	// IPv4 Services
	go r.detectPublicIP(4, []string{
		"https://v4.ident.me",
		"https://api4.ipify.org",
		"https://ifconfig.me/ip",
	}, &wg)

	// IPv6 Services
	go r.detectPublicIP(6, []string{
		"https://v6.ident.me",
		"https://api6.ipify.org",
		"https://ifconfig.co/ip",
	}, &wg)

	wg.Wait()
}

func (r *Resolver) detectPublicIP(family int, services []string, wg *sync.WaitGroup) {
	defer wg.Done()
	client := &http.Client{Timeout: 5 * time.Second}

	var detectedIP net.IP
	for _, svc := range services {
		resp, err := client.Get(svc)
		if err != nil {
			continue
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		ip := net.ParseIP(strings.TrimSpace(string(data)))
		if ip != nil {
			if (family == 4 && ip.To4() != nil) || (family == 6 && ip.To4() == nil) {
				detectedIP = ip
				break
			}
		}
	}

	if detectedIP == nil {
		logger.DNS().Warn("dns auto ECS public IP detection failed", "family", family)
		return
	}

	r.autoECSNetMu.Lock()
	defer r.autoECSNetMu.Unlock()

	if family == 4 {
		r.autoECSNet4 = &net.IPNet{IP: detectedIP.Mask(net.CIDRMask(24, 32)), Mask: net.CIDRMask(24, 32)}
	} else {
		r.autoECSNet6 = &net.IPNet{IP: detectedIP.Mask(net.CIDRMask(48, 128)), Mask: net.CIDRMask(48, 128)}
	}
	logger.DNS().Info("dns auto ECS initialized", "family", family)
}

// Resolve resolves a hostname to an IP address, utilizing rules, cache, and upstreams.
func (r *Resolver) Resolve(ctx context.Context, host string, clientIP net.IP) (string, error) {
	target := host
	if r.Rules != nil {
		if v, ok := r.Rules.GetHost(host); ok && v != "" {
			target = v
		}
	}

	// Hosts rules may store IPv6 as "[2001:db8::1]" (bracket form, common in
	// host:port lists). net.ParseIP rejects brackets, so strip them first.
	if ip := parseHostIP(target); ip != nil {
		return ip.String(), nil
	}

	if r.backend == nil {
		return r.resolveSystem(ctx, host, target)
	}

	ip, err := r.resolveUpstream(ctx, target, clientIP)
	if err == nil {
		return ip, nil
	}

	logger.DNS().Debug("dns upstreams failed, falling back to system DNS", "target", target, "error", err)
	return r.resolveSystem(ctx, host, target)
}

// parseHostIP parses an IP literal, accepting optional surrounding brackets
// used for IPv6 in Hosts rules (e.g. "[2001:db8::1]"). Returns nil if s is not
// an IP address.
func parseHostIP(s string) net.IP {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '[' && s[len(s)-1] == ']' {
		s = s[1 : len(s)-1]
	}
	return net.ParseIP(s)
}

func (r *Resolver) resolveSystem(ctx context.Context, host, target string) (string, error) {
	// Check cache for system results as well (use type 0 for system)
	if ip, ok := r.getCache(target, 0); ok {
		return ip, nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	ch := r.systemSF.DoChan(target, func() (any, error) {
		if ip, ok := r.getCache(target, 0); ok {
			return ip, nil
		}
		flightCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.queryTimeout())
		defer cancel()

		ips, err := net.DefaultResolver.LookupHost(flightCtx, target)
		if err != nil {
			return nil, fmt.Errorf("dns: could not resolve %s: %w", host, err)
		}

		selectedIP := ips[0]
		if r.Config != nil && r.Config.IPv6 {
			for _, ip := range ips {
				if net.ParseIP(ip).To4() == nil {
					selectedIP = ip
					break
				}
			}
		}
		logger.DNS().Debug("dns system resolved", "host", host, "ip", selectedIP)
		// System resolver doesn't expose TTL, use a conservative default (5m)
		r.setCache(target, selectedIP, 0, 300)
		return selectedIP, nil
	})

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			return "", res.Err
		}
		return res.Val.(string), nil
	}
}

// resolveUpstream resolves via the configured DNS upstreams, honoring the IP
// preference mode. Results flow through the shared DNS cache (lookupType).
func (r *Resolver) resolveUpstream(ctx context.Context, target string, clientIP net.IP) (string, error) {
	// IPv6 disabled -> IPv4 only
	if !r.Config.IPv6 {
		ip, _, err := r.lookupType(ctx, target, dns.TypeA, clientIP)
		return ip, err
	}

	switch r.Config.Preference.Mode {
	case config.IPPreferenceIPv4:
		ip, _, err := r.lookupType(ctx, target, dns.TypeA, clientIP)
		return ip, err
	default: // standard, ipv6, or unknown
		ip, _, err := r.resolveStandard(ctx, target, clientIP)
		return ip, err
	}
}

// resolveStandard performs the standard resolution: if IPv6 is enabled, try AAAA first then fallback to A.
func (r *Resolver) resolveStandard(ctx context.Context, target string, clientIP net.IP) (string, uint32, error) {
	if r.Config.IPv6 {
		if ip, ttl, err := r.lookupType(ctx, target, dns.TypeAAAA, clientIP); err == nil {
			return ip, ttl, err
		}
	}
	return r.lookupType(ctx, target, dns.TypeA, clientIP)
}

func (r *Resolver) queryTimeout() time.Duration {
	if r.Config != nil && r.Config.Timeout.DNS > 0 {
		return time.Duration(r.Config.Timeout.DNS) * time.Second
	}
	return 5 * time.Second
}

func (r *Resolver) negCacheTTL() time.Duration {
	if r.negativeCacheTTL > 0 {
		return r.negativeCacheTTL
	}
	return 60 * time.Second
}

func (r *Resolver) negativeTTLFromMsg(reply *dns.Msg) time.Duration {
	if r.negativeCacheTTL > 0 {
		return r.negativeCacheTTL
	}
	const maxNegTTL = 60 * time.Second
	const minNegTTL = 5 * time.Second
	if reply != nil {
		for _, ns := range reply.Ns {
			if soa, ok := ns.(*dns.SOA); ok {
				ttl := time.Duration(soa.Minttl) * time.Second
				if ttl <= 0 {
					return minNegTTL
				}
				if ttl > maxNegTTL {
					return maxNegTTL
				}
				return ttl
			}
		}
	}
	return maxNegTTL
}

// queryDNS performs a DNS query and returns all matching records along with the upstream address.
func (r *Resolver) queryDNS(ctx context.Context, target string, qType uint16, clientIP net.IP) ([]ipRecord, string, error) {
	m := r.buildMessage(target, qType, clientIP)
	var (
		reply *dns.Msg
		addr  string
		err   error
	)
	if cb, ok := r.backend.(dnsContextBackend); ok {
		reply, addr, err = cb.ExchangeContext(ctx, m)
	} else {
		reply, addr, err = r.backend.Exchange(m)
	}
	if err != nil {
		return nil, "", err
	}
	if reply.Rcode != dns.RcodeSuccess {
		err := fmt.Errorf("dns rcode %s from %s", dns.RcodeToString[reply.Rcode], addr)
		if reply.Rcode == dns.RcodeNameError {
			return nil, "", &negativeDNSError{ttl: r.negativeTTLFromMsg(reply), err: err}
		}
		return nil, "", err
	}
	var records []ipRecord
	for _, ans := range reply.Answer {
		switch qType {
		case dns.TypeAAAA:
			if aaaa, ok := ans.(*dns.AAAA); ok {
				records = append(records, ipRecord{ip: aaaa.AAAA.String(), ttl: aaaa.Hdr.Ttl})
			}
		case dns.TypeA:
			if a, ok := ans.(*dns.A); ok {
				records = append(records, ipRecord{ip: a.A.String(), ttl: a.Hdr.Ttl})
			}
		}
	}
	if len(records) == 0 {
		return nil, "", &negativeDNSError{
			ttl: r.negativeTTLFromMsg(reply),
			err: fmt.Errorf("no records of type %d found", qType),
		}
	}
	return records, addr, nil
}

type lookupResult struct {
	ip  string
	ttl uint32
}

func sfLookupKey(host string, qType uint16) string {
	return host + ":" + strconv.FormatUint(uint64(qType), 10)
}

// lookupType returns the first A or AAAA record for the target. It consults
// the shared DNS cache first and stores upstream answers under the record's
// TTL, so repeated lookups do not re-query upstreams.
func (r *Resolver) lookupType(ctx context.Context, target string, qType uint16, clientIP net.IP) (string, uint32, error) {
	if ip, neg, ok := r.getCacheEntry(target, qType); ok {
		if neg {
			return "", 0, errNegativeCacheHit
		}
		return ip, 0, nil
	}
	if err := ctx.Err(); err != nil {
		return "", 0, err
	}

	sfKey := sfLookupKey(target, qType)
	ch := r.lookupSF.DoChan(sfKey, func() (any, error) {
		if ip, neg, ok := r.getCacheEntry(target, qType); ok {
			if neg {
				return nil, errNegativeCacheHit
			}
			return lookupResult{ip: ip, ttl: 0}, nil
		}

		// Detached bounded timeout prevents one caller's cancellation from aborting peers.
		flightCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.queryTimeout())
		defer cancel()

		records, addr, err := r.queryDNS(flightCtx, target, qType, clientIP)
		if err != nil {
			var negErr *negativeDNSError
			if errors.As(err, &negErr) {
				r.setNegativeCache(target, qType, negErr.ttl)
			}
			return nil, err
		}
		first := records[0]
		logger.DNS().Debug("dns query resolved", "target", target, "ip", first.ip, "type", dns.TypeToString[qType], "ttl", first.ttl, "upstream", addr)
		r.setCache(target, first.ip, qType, first.ttl)
		return lookupResult(first), nil
	})

	select {
	case <-ctx.Done():
		return "", 0, ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			return "", 0, res.Err
		}
		lr := res.Val.(lookupResult)
		return lr.ip, lr.ttl, nil
	}
}

func (r *Resolver) buildMessage(target string, qType uint16, clientIP net.IP) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(target), qType)
	m.Id = dns.Id()
	m.RecursionDesired = true

	if r.Config != nil && r.Config.ECS != "" {
		if ecs := r.getECS(qType, clientIP); ecs != nil {
			o := m.IsEdns0()
			if o == nil {
				m.SetEdns0(1232, false)
				o = m.IsEdns0()
			}
			o.Option = append(o.Option, ecs)
		}
	}
	return m
}

func (r *Resolver) initStaticECS() {
	r.staticECSOnce.Do(func() {
		if r.Config == nil || r.Config.ECS == "" || r.Config.ECS == "auto" {
			return
		}
		_, parsed, err := net.ParseCIDR(r.Config.ECS)
		if err == nil {
			r.staticECSNet = parsed
		}
	})
}

func (r *Resolver) getECS(qType uint16, clientIP net.IP) *dns.EDNS0_SUBNET {
	var ipNet *net.IPNet

	if r.Config.ECS == "auto" {
		r.autoECSNetMu.RLock()
		if qType == dns.TypeAAAA {
			ipNet = r.autoECSNet6
			if ipNet == nil {
				ipNet = r.autoECSNet4
			}
		} else {
			ipNet = r.autoECSNet4
			if ipNet == nil {
				ipNet = r.autoECSNet6
			}
		}
		r.autoECSNetMu.RUnlock()

		if ipNet == nil && clientIP != nil {
			if !clientIP.IsLoopback() && !clientIP.IsPrivate() && !clientIP.IsLinkLocalUnicast() {
				if ip4 := clientIP.To4(); ip4 != nil {
					ipNet = &net.IPNet{IP: ip4.Mask(net.CIDRMask(24, 32)), Mask: net.CIDRMask(24, 32)}
				} else {
					ipNet = &net.IPNet{IP: clientIP.Mask(net.CIDRMask(48, 128)), Mask: net.CIDRMask(48, 128)}
				}
			}
		}
	} else {
		r.initStaticECS()
		ipNet = r.staticECSNet
	}

	if ipNet == nil {
		return nil
	}

	ones, _ := ipNet.Mask.Size()
	e := &dns.EDNS0_SUBNET{
		Code:          dns.EDNS0SUBNET,
		SourceNetmask: uint8(ones),
		SourceScope:   0,
	}
	if ip4 := ipNet.IP.To4(); ip4 != nil {
		e.Family = 1
		e.Address = ip4
	} else {
		e.Family = 2
		e.Address = ipNet.IP
	}

	return e
}

func (r *Resolver) getCache(host string, qType uint16) (string, bool) {
	ip, neg, ok := r.getCacheEntry(host, qType)
	if !ok || neg {
		return "", false
	}
	return ip, true
}

func (r *Resolver) getCacheEntry(host string, qType uint16) (string, bool, bool) {
	key := dnsCacheKey{host: host, qType: qType}
	now := time.Now()

	r.cacheMu.RLock()
	entry := r.cache[key]
	if entry != nil && now.Before(entry.expiresAt) {
		entry.lastAccessed.Store(now.UnixNano())
		ip := entry.ip
		neg := entry.negative
		r.cacheMu.RUnlock()
		return ip, neg, true
	}
	r.cacheMu.RUnlock()
	return "", false, false
}

func (r *Resolver) setCache(host, ip string, qType uint16, ttl uint32) {
	if ttl == 0 {
		ttl = 60 // Minimum 1m
	}
	if ttl > 86400 {
		ttl = 86400 // Maximum 24h
	}
	r.putCacheEntry(dnsCacheKey{host: host, qType: qType}, ip, false, time.Duration(ttl)*time.Second)
}

func (r *Resolver) setNegativeCache(host string, qType uint16, ttl time.Duration) {
	if ttl <= 0 {
		ttl = r.negCacheTTL()
	}
	r.putCacheEntry(dnsCacheKey{host: host, qType: qType}, "", true, ttl)
}

func (r *Resolver) putCacheEntry(key dnsCacheKey, ip string, negative bool, ttl time.Duration) {
	now := time.Now()
	entry := &cacheEntry{
		ip:        ip,
		negative:  negative,
		expiresAt: now.Add(ttl),
	}
	entry.lastAccessed.Store(now.UnixNano())

	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()

	if r.cache == nil {
		r.cache = make(map[dnsCacheKey]*cacheEntry)
	}

	limit := 10000
	if r.Config != nil && r.Config.Limit.DNSCacheSize > 0 {
		limit = r.Config.Limit.DNSCacheSize
	}

	if _, exists := r.cache[key]; !exists && len(r.cache) >= limit {
		r.evictSampledLocked(now, limit)
	}

	r.cache[key] = entry
}

// evictSampledLocked performs bounded O(1) sampled LRU eviction under cacheMu.Lock.
func (r *Resolver) evictSampledLocked(now time.Time, limit int) {
	const sampleLimit = 32
	const maxIter = 64

	var oldestKey dnsCacheKey
	var oldestTime int64
	hasOldest := false
	sampled := 0
	iters := 0

	for k, v := range r.cache {
		iters++
		if !now.Before(v.expiresAt) {
			delete(r.cache, k)
			if len(r.cache) < limit {
				return
			}
			if iters >= maxIter {
				break
			}
			continue
		}
		t := v.lastAccessed.Load()
		if !hasOldest || t < oldestTime {
			oldestKey = k
			oldestTime = t
			hasOldest = true
		}
		sampled++
		if sampled >= sampleLimit || iters >= maxIter {
			break
		}
	}
	if hasOldest {
		delete(r.cache, oldestKey)
	}
}

func (r *Resolver) cleanCacheRoutine() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-r.stopChan:
			return
		case <-ticker.C:
			r.cacheMu.Lock()
			now := time.Now()
			for k, v := range r.cache {
				if !now.Before(v.expiresAt) {
					delete(r.cache, k)
				}
			}
			remaining := len(r.cache)
			r.cacheMu.Unlock()
			logger.DNS().Debug("dns cache cleanup completed", "entries", remaining)
		}
	}
}

func (r *Resolver) Invalidate(host string) {
	r.cacheMu.Lock()
	delete(r.cache, dnsCacheKey{host: host, qType: dns.TypeA})
	delete(r.cache, dnsCacheKey{host: host, qType: dns.TypeAAAA})
	delete(r.cache, dnsCacheKey{host: host, qType: 0}) // System DNS cache
	r.cacheMu.Unlock()

	r.lookupSF.Forget(sfLookupKey(host, dns.TypeA))
	r.lookupSF.Forget(sfLookupKey(host, dns.TypeAAAA))
	r.systemSF.Forget(host)

	logger.DNS().Debug("dns cache invalidated", "host", host)
}

// Close gracefully shuts down the resolver, stopping background routines.
func (r *Resolver) Close() error {
	close(r.stopChan)
	if c, ok := r.backend.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// Backend returns the underlying dnsBackend for direct DNS message exchange.
// This is useful for forwarding raw DNS queries (e.g., from a TUN interface).
func (r *Resolver) Backend() dnsBackend {
	return r.backend
}
