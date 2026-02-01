package dns

import (
	"snirect/internal/config"
	"snirect/internal/logger"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type cacheEntry struct {
	ip        string
	expiresAt time.Time
}

type Resolver struct {
	Rules       *config.Rules
	nameservers []string // formatted as addr:port
	dotServers  []string // DoT servers
	
	cache   map[string]cacheEntry
	cacheMu sync.RWMutex
}

const defaultTTL = 24 * time.Hour

func NewResolver(rules *config.Rules) *Resolver {
	r := &Resolver{
		Rules: rules,
		cache: make(map[string]cacheEntry),
	}
	for _, ns := range rules.DNS.Nameserver {
		if strings.HasPrefix(ns, "tls://") {
			addr := strings.TrimPrefix(ns, "tls://")
			if !strings.Contains(addr, ":") {
				addr += ":853"
			}
			r.dotServers = append(r.dotServers, addr)
		} else if strings.HasPrefix(ns, "https://") {
			// DoH is complex to implement from scratch here, skip for now
		} else {
			addr := ns
			if !strings.Contains(addr, ":") {
				addr += ":53"
			}
			r.nameservers = append(r.nameservers, addr)
		}
	}
	
	// Start cleaner
	go r.cleanCacheRoutine()
	
	return r
}

func (r *Resolver) Resolve(ctx context.Context, host string) (string, error) {
	target := host

	// 1. 检查 hosts 映射
	matched := false
	if v, ok := r.Rules.Hosts[host]; ok {
		target = v
		matched = true
	} else {
		// 检查通配符
		for k, v := range r.Rules.Hosts {
			if strings.HasPrefix(k, ".") && strings.HasSuffix(host, k) {
				target = v
				matched = true
				break
			}
		}
	}

	// 如果映射结果直接就是 IP，直接返回
	if matched && net.ParseIP(target) != nil {
		return target, nil
	}

	// Check Cache
	if ip, ok := r.getCache(target); ok {
		logger.Debug("Resolved %s -> %s from cache", host, ip)
		return ip, nil
	}

	// 2. 使用外部 DNS 查询 target (可能是映射后的域名，也可能是原始域名)
	logger.Debug("Resolving %s (originally %s) via external DNS", target, host)

	// DoT 查询
	if len(r.dotServers) > 0 {
		for _, addr := range r.dotServers {
			ip, err := r.queryExternal(target, "tcp-tls", addr)
			if err == nil {
				logger.Debug("Resolved %s -> %s via DoT %s", host, ip, addr)
				r.setCache(target, ip)
				return ip, nil
			}
		}
	}

	// UDP 查询
	for _, addr := range r.nameservers {
		ip, err := r.queryExternal(target, "udp", addr)
		if err == nil {
			logger.Debug("Resolved %s -> %s via UDP %s", host, ip, addr)
			r.setCache(target, ip)
			return ip, nil
		}
	}

	// 3. 最终回退到系统 DNS
	ips, err := net.DefaultResolver.LookupHost(ctx, target)
	if err == nil && len(ips) > 0 {
		logger.Debug("Resolved %s -> %s via System DNS", host, ips[0])
		r.setCache(target, ips[0])
		return ips[0], nil
	}

	return "", fmt.Errorf("could not resolve host: %s", host)
}

func (r *Resolver) getCache(host string) (string, bool) {
	r.cacheMu.RLock()
	defer r.cacheMu.RUnlock()
	entry, ok := r.cache[host]
	if !ok {
		return "", false
	}
	if time.Now().After(entry.expiresAt) {
		return "", false
	}
	return entry.ip, true
}

func (r *Resolver) setCache(host, ip string) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()
	r.cache[host] = cacheEntry{
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
	delete(r.cache, host)
	logger.Debug("Invalidated DNS cache for %s", host)
}

func (r *Resolver) queryExternal(host, network, addr string) (string, error) {
	c := &dns.Client{
		Net:     network,
		Timeout: 2 * time.Second,
	}
	if network == "tcp-tls" {
		c.TLSConfig = &tls.Config{InsecureSkipVerify: false}
	}

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(host), dns.TypeA)
	r_msg, _, err := c.Exchange(m, addr)
	if err != nil {
		return "", err
	}
	if r_msg.Rcode != dns.RcodeSuccess {
		return "", fmt.Errorf("dns query failed with rcode %d", r_msg.Rcode)
	}
	for _, ans := range r_msg.Answer {
		if a, ok := ans.(*dns.A); ok {
			return a.A.String(), nil
		}
	}
	return "", fmt.Errorf("no A record found")
}
