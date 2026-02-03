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

	"github.com/AdguardTeam/dnsproxy/upstream"
	"github.com/miekg/dns"
)

type cacheEntry struct {
	ip        string
	expiresAt time.Time
}

type Resolver struct {
	Config    *config.Config
	Rules     *config.Rules
	upstreams []upstream.Upstream

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

func NewResolver(cfg *config.Config, rules *config.Rules) *Resolver {
	r := &Resolver{
		Config: cfg,
		Rules:  rules,
		cache:  make(map[string]cacheEntry),
	}

	// Completely silence library logs
	libLogger := slog.New(&discardHandler{})

	opts := &upstream.Options{
		Timeout: 5 * time.Second,
		Logger:  libLogger,
	}

	if len(cfg.DNS.BootstrapDNS) > 0 {
		var bootstrapResolvers []upstream.Resolver
		for _, bootAddr := range cfg.DNS.BootstrapDNS {
			bootRes, err := upstream.NewUpstreamResolver(bootAddr, &upstream.Options{
				Timeout: 3 * time.Second,
				Logger:  libLogger,
			})
			if err != nil {
				continue
			}
			bootstrapResolvers = append(bootstrapResolvers, bootRes)
		}

		if len(bootstrapResolvers) > 0 {
			opts.Bootstrap = upstream.ParallelResolver(bootstrapResolvers)
		}
	}

	for _, ns := range cfg.DNS.Nameserver {
		u, err := upstream.AddressToUpstream(ns, opts)
		if err != nil {
			logger.Warn("DNS: failed to create upstream %s: %v", ns, err)
			continue
		}
		r.upstreams = append(r.upstreams, u)
	}

	go r.cleanCacheRoutine()

	if cfg.ECS == "auto" {
		go r.initAutoECS()
	}

	return r
}

func (r *Resolver) initAutoECS() {
	var wg sync.WaitGroup
	wg.Add(2)

	detect := func(family int, services []string) {
		defer wg.Done()
		client := &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
			},
		}
		var detectedIP net.IP

		for _, svc := range services {
			req, _ := http.NewRequest("GET", svc, nil)
			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			data, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				continue
			}
			ipStr := strings.TrimSpace(string(data))
			ip := net.ParseIP(ipStr)
			if ip != nil {
				if family == 4 && ip.To4() != nil {
					detectedIP = ip
					break
				} else if family == 6 && ip.To4() == nil {
					detectedIP = ip
					break
				}
			}
		}

		if detectedIP != nil {
			var ipNet *net.IPNet
			if family == 4 {
				ip4 := detectedIP.To4()
				ipNet = &net.IPNet{
					IP:   ip4.Mask(net.CIDRMask(24, 32)),
					Mask: net.CIDRMask(24, 32),
				}
				r.autoECSNetMu.Lock()
				r.autoECSNet4 = ipNet
				r.autoECSNetMu.Unlock()
			} else {
				ipNet = &net.IPNet{
					IP:   detectedIP.Mask(net.CIDRMask(48, 128)),
					Mask: net.CIDRMask(48, 128),
				}
				r.autoECSNetMu.Lock()
				r.autoECSNet6 = ipNet
				r.autoECSNetMu.Unlock()
			}
			logger.Info("DNS: Auto ECS initialized (IPv%d): %s/%d", family, ipNet.IP, func() int {
				ones, _ := ipNet.Mask.Size()
				return ones
			}())
		} else {
			logger.Warn("DNS: Failed to detect public IPv%d for auto ECS", family)
		}
	}

	// IPv4 Services
	go detect(4, []string{
		"https://v4.ident.me",
		"https://api4.ipify.org",
		"https://ifconfig.me/ip",
	})

	// IPv6 Services
	go detect(6, []string{
		"https://v6.ident.me",
		"https://api6.ipify.org",
		"https://ifconfig.co/ip",
	})

	wg.Wait()
}

func (r *Resolver) Resolve(ctx context.Context, host string, clientIP net.IP) (string, error) {
	target := host
	if v, ok := r.Rules.GetHost(host); ok && v != "" {
		target = v
	}

	if net.ParseIP(target) != nil {
		return target, nil
	}

	if len(r.upstreams) == 0 {
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
	r.setCache(target, selectedIP, 0)
	return selectedIP, nil
}

func (r *Resolver) resolveStrictIPv6(ctx context.Context, target string, clientIP net.IP) (string, error) {
	if r.Config.IPv6 {
		if ip, ok := r.getCache(target, dns.TypeAAAA); ok {
			logger.Debug("DNS: %s -> %s (AAAA) from cache", target, ip)
			return ip, nil
		}

		ip, err := r.lookupType(ctx, target, dns.TypeAAAA, clientIP)
		if err == nil {
			r.setCache(target, ip, dns.TypeAAAA)
			return ip, nil
		}
		logger.Debug("DNS: AAAA lookup failed for %s: %v, falling back to A", target, err)
	}

	if ip, ok := r.getCache(target, dns.TypeA); ok {
		logger.Debug("DNS: %s -> %s (A) from cache", target, ip)
		return ip, nil
	}

	ip, err := r.lookupType(ctx, target, dns.TypeA, clientIP)
	if err == nil {
		r.setCache(target, ip, dns.TypeA)
		return ip, nil
	}

	return "", err
}

func (r *Resolver) lookupType(ctx context.Context, target string, qType uint16, clientIP net.IP) (string, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(target), qType)
	m.Id = dns.Id()
	m.RecursionDesired = true

	// Inject EDNS Client Subnet (ECS) if configured
	if r.Config.ECS != "" {
		var ipNet *net.IPNet
		var err error

		if r.Config.ECS == "auto" {
			r.autoECSNetMu.RLock()
			// Prefer matching family for query type
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
						ipNet = &net.IPNet{
							IP:   ip4.Mask(net.CIDRMask(24, 32)),
							Mask: net.CIDRMask(24, 32),
						}
					} else {
						ipNet = &net.IPNet{
							IP:   clientIP.Mask(net.CIDRMask(48, 128)),
							Mask: net.CIDRMask(48, 128),
						}
					}
				}
			}

			// For auto mode, we use full mask (32/128) to trigger better upstream optimization
			// while the IP itself is already masked for privacy.
			if ipNet != nil {
				e := &dns.EDNS0_SUBNET{
					Code:        dns.EDNS0SUBNET,
					SourceScope: 0,
				}
				if ip4 := ipNet.IP.To4(); ip4 != nil {
					e.Family = 1
					e.Address = ip4
					e.SourceNetmask = 32
				} else {
					e.Family = 2
					e.Address = ipNet.IP
					e.SourceNetmask = 128
				}

				o := m.IsEdns0()
				if o == nil {
					m.SetEdns0(1232, false)
					o = m.IsEdns0()
				}
				o.Option = append(o.Option, e)
				logger.Debug("DNS: Sending ECS (%d) for %s: %s/%d", e.Family, target, e.Address, e.SourceNetmask)
			}
		} else {
			_, ipNet, err = net.ParseCIDR(r.Config.ECS)
			if err == nil && ipNet != nil {
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
				o := m.IsEdns0()
				if o == nil {
					m.SetEdns0(1232, false)
					o = m.IsEdns0()
				}
				o.Option = append(o.Option, e)
			}
		}
	}

	reply, resolvedUpstream, err := upstream.ExchangeParallel(r.upstreams, m)

	if err != nil {
		return "", err
	}
	if reply.Rcode != dns.RcodeSuccess {
		return "", fmt.Errorf("dns rcode %s from %s", dns.RcodeToString[reply.Rcode], resolvedUpstream.Address())
	}

	// Log ECS response scope if present
	// if opt := reply.IsEdns0(); opt != nil {
	// 	for _, o := range opt.Option {
	// 		if ecs, ok := o.(*dns.EDNS0_SUBNET); ok {
	// 			logger.Debug("DNS: ECS response from %s: Scope /%d", resolvedUpstream.Address(), ecs.SourceScope)
	// 		}
	// 	}
	// }

	for _, ans := range reply.Answer {
		if qType == dns.TypeAAAA {
			if aaaa, ok := ans.(*dns.AAAA); ok {
				ip := aaaa.AAAA.String()
				logger.Debug("DNS: %s -> %s (AAAA) via %s", target, ip, resolvedUpstream.Address())
				return ip, nil
			}
		} else {
			if a, ok := ans.(*dns.A); ok {
				ip := a.A.String()
				logger.Debug("DNS: %s -> %s (A) via %s", target, ip, resolvedUpstream.Address())
				return ip, nil
			}
		}
	}

	return "", fmt.Errorf("no records of type %d found", qType)
}

func (r *Resolver) getCache(host string, qType uint16) (string, bool) {
	r.cacheMu.RLock()
	defer r.cacheMu.RUnlock()
	key := fmt.Sprintf("%s:%d", host, qType)
	if entry, ok := r.cache[key]; ok && time.Now().Before(entry.expiresAt) {
		return entry.ip, true
	}
	return "", false
}

func (r *Resolver) setCache(host, ip string, qType uint16) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()
	key := fmt.Sprintf("%s:%d", host, qType)
	r.cache[key] = cacheEntry{
		ip:        ip,
		expiresAt: time.Now().Add(defaultTTL),
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

	// Cache keys are in format "host:type", so we must iterate to find all matches
	prefix := host + ":"
	for key := range r.cache {
		if strings.HasPrefix(key, prefix) {
			delete(r.cache, key)
		}
	}
	logger.Debug("DNS: Cache invalidated for %s", host)
}
