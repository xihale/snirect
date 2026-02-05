package dns

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"snirect/internal/config"
	"snirect/internal/logger"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type dnsBackend interface {
	Exchange(m *dns.Msg) (*dns.Msg, string, error)
}

type cacheEntry struct {
	ip        string
	expiresAt time.Time
}

type Resolver struct {
	Config  *config.Config
	Rules   *config.Rules
	backend dnsBackend

	cache   map[string]cacheEntry
	cacheMu sync.RWMutex

	autoECSNet4  *net.IPNet
	autoECSNet6  *net.IPNet
	autoECSNetMu sync.RWMutex
}

const defaultTTL = 24 * time.Hour

// discardHandler silently drops all logs
type discardHandler struct{}

func (h *discardHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return false
}

func (h *discardHandler) Handle(ctx context.Context, r slog.Record) error {
	return nil
}

func (h *discardHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *discardHandler) WithGroup(name string) slog.Handler {
	return h
}

// NewResolver creates a new Resolver instance based on the provided configuration.
func NewResolver(cfg *config.Config, rules *config.Rules) *Resolver {
	r := &Resolver{
		Config: cfg,
		Rules:  rules,
		cache:  make(map[string]cacheEntry),
	}

	r.backend = newBackend(cfg, rules)

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
		logger.Warn("DNS: Failed to detect public IPv%d for auto ECS", family)
		return
	}

	r.autoECSNetMu.Lock()
	defer r.autoECSNetMu.Unlock()

	if family == 4 {
		r.autoECSNet4 = &net.IPNet{IP: detectedIP.Mask(net.CIDRMask(24, 32)), Mask: net.CIDRMask(24, 32)}
	} else {
		r.autoECSNet6 = &net.IPNet{IP: detectedIP.Mask(net.CIDRMask(48, 128)), Mask: net.CIDRMask(48, 128)}
	}
	logger.Info("DNS: Auto ECS initialized (IPv%d): %s", family, detectedIP)
}

// Resolve resolves a hostname to an IP address, utilizing rules, cache, and upstreams.
func (r *Resolver) Resolve(ctx context.Context, host string, clientIP net.IP) (string, error) {
	target := host
	if v, ok := r.Rules.GetHost(host); ok && v != "" {
		target = v
	}

	if net.ParseIP(target) != nil {
		return target, nil
	}

	if r.backend == nil {
		return r.resolveSystem(ctx, host, target)
	}

	ip, err := r.resolveStrictIPv6(ctx, target, clientIP)
	if err == nil {
		return ip, nil
	}

	logger.Debug("DNS: All upstreams failed for %s: %v. Falling back to System DNS.", target, err)
	return r.resolveSystem(ctx, host, target)
}

func (r *Resolver) resolveSystem(ctx context.Context, host, target string) (string, error) {
	// Check cache for system results as well (use type 0 for system)
	if ip, ok := r.getCache(target, 0); ok {
		return ip, nil
	}

	ips, err := net.DefaultResolver.LookupHost(ctx, target)
	if err != nil {
		return "", fmt.Errorf("dns: could not resolve %s: %w", host, err)
	}

	selectedIP := ips[0]
	if r.Config.IPv6 {
		for _, ip := range ips {
			if net.ParseIP(ip).To4() == nil {
				selectedIP = ip
				break
			}
		}
	}
	logger.Debug("DNS: %s -> %s (System DNS)", host, selectedIP)
	// System resolver doesn't expose TTL, use a conservative default (5m)
	r.setCache(target, selectedIP, 0, 300)
	return selectedIP, nil
}

func (r *Resolver) resolveStrictIPv6(ctx context.Context, target string, clientIP net.IP) (string, error) {
	if r.Config.IPv6 {
		if ip, ok := r.getCache(target, dns.TypeAAAA); ok {
			logger.Debug("DNS: %s -> %s (AAAA) from cache", target, ip)
			return ip, nil
		}

		ip, ttl, err := r.lookupType(ctx, target, dns.TypeAAAA, clientIP)
		if err == nil {
			r.setCache(target, ip, dns.TypeAAAA, ttl)
			return ip, nil
		}
		logger.Debug("DNS: AAAA lookup failed for %s: %v, falling back to A", target, err)
	}

	if ip, ok := r.getCache(target, dns.TypeA); ok {
		logger.Debug("DNS: %s -> %s (A) from cache", target, ip)
		return ip, nil
	}

	ip, ttl, err := r.lookupType(ctx, target, dns.TypeA, clientIP)
	if err == nil {
		r.setCache(target, ip, dns.TypeA, ttl)
		return ip, nil
	}

	return "", err
}

func (r *Resolver) lookupType(ctx context.Context, target string, qType uint16, clientIP net.IP) (string, uint32, error) {
	m := r.buildMessage(target, qType, clientIP)

	reply, addr, err := r.backend.Exchange(m)
	if err != nil {
		return "", 0, err
	}
	if reply.Rcode != dns.RcodeSuccess {
		return "", 0, fmt.Errorf("dns rcode %s from %s", dns.RcodeToString[reply.Rcode], addr)
	}

	ip, ttl, err := r.parseReply(reply, qType)
	if err == nil {
		logger.Debug("DNS: %s -> %s (%s, TTL: %d) via %s", target, ip, dns.TypeToString[qType], ttl, addr)
	}
	return ip, ttl, err
}

func (r *Resolver) buildMessage(target string, qType uint16, clientIP net.IP) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(target), qType)
	m.Id = dns.Id()
	m.RecursionDesired = true

	if r.Config.ECS != "" {
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
		_, parsed, err := net.ParseCIDR(r.Config.ECS)
		if err == nil {
			ipNet = parsed
		}
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

	// For auto mode, we use full mask to trigger better upstream optimization
	if r.Config.ECS == "auto" {
		if e.Family == 1 {
			e.SourceNetmask = 32
		} else {
			e.SourceNetmask = 128
		}
	}

	return e
}

func (r *Resolver) parseReply(reply *dns.Msg, qType uint16) (string, uint32, error) {
	for _, ans := range reply.Answer {
		if qType == dns.TypeAAAA {
			if aaaa, ok := ans.(*dns.AAAA); ok {
				return aaaa.AAAA.String(), aaaa.Hdr.Ttl, nil
			}
		} else {
			if a, ok := ans.(*dns.A); ok {
				return a.A.String(), a.Hdr.Ttl, nil
			}
		}
	}
	return "", 0, fmt.Errorf("no records of type %d found", qType)
}

func (r *Resolver) cacheKey(host string, qType uint16) string {
	return fmt.Sprintf("%s:%d", host, qType)
}

func (r *Resolver) getCache(host string, qType uint16) (string, bool) {
	r.cacheMu.RLock()
	defer r.cacheMu.RUnlock()
	if entry, ok := r.cache[r.cacheKey(host, qType)]; ok && time.Now().Before(entry.expiresAt) {
		return entry.ip, true
	}
	return "", false
}

func (r *Resolver) setCache(host, ip string, qType uint16, ttl uint32) {
	if ttl == 0 {
		ttl = 60 // Minimum 1m
	}
	if ttl > 86400 {
		ttl = 86400 // Maximum 24h
	}

	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()

	limit := r.Config.Limit.DNSCacheSize
	if limit <= 0 {
		limit = 10000
	}

	if len(r.cache) >= limit {
		// Simple random eviction to make space
		for k := range r.cache {
			delete(r.cache, k)
			break
		}
	}

	r.cache[r.cacheKey(host, qType)] = cacheEntry{
		ip:        ip,
		expiresAt: time.Now().Add(time.Duration(ttl) * time.Second),
	}
}

func (r *Resolver) cleanCacheRoutine() {
	ticker := time.NewTicker(10 * time.Minute)
	for range ticker.C {
		r.cacheMu.Lock()
		now := time.Now()
		for k, v := range r.cache {
			if now.After(v.expiresAt) {
				delete(r.cache, k)
			}
		}
		r.cacheMu.Unlock()
	}
}

func (r *Resolver) Invalidate(host string) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()

	delete(r.cache, r.cacheKey(host, dns.TypeA))
	delete(r.cache, r.cacheKey(host, dns.TypeAAAA))
	delete(r.cache, r.cacheKey(host, 0)) // System DNS cache

	logger.Debug("DNS: Cache invalidated for %s", host)
}
