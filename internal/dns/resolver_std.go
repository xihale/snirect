package dns

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/xihale/snirect/internal/config"
	"github.com/xihale/snirect/internal/logger"
	"github.com/xihale/snirect/internal/rules"
	"golang.org/x/sync/singleflight"
)

type stdBackend struct {
	upstreams []stdUpstream
	timeout   time.Duration
}

type stdUpstream interface {
	Exchange(m *dns.Msg) (*dns.Msg, error)
	Address() string
}

type stdContextUpstream interface {
	ExchangeContext(ctx context.Context, m *dns.Msg) (*dns.Msg, error)
}

func exchangeUpstream(ctx context.Context, u stdUpstream, m *dns.Msg) (*dns.Msg, error) {
	if cu, ok := u.(stdContextUpstream); ok {
		return cu.ExchangeContext(ctx, m)
	}
	return u.Exchange(m)
}

// exchangeParallel sends the same DNS query to multiple upstreams concurrently.
// It returns the first successful reply, or the last error if all upstreams fail.
// The caller provides a context for cancellation and timeout control.
func exchangeParallel(ctx context.Context, m *dns.Msg, upstreams []stdUpstream) (*dns.Msg, string, error) {
	if len(upstreams) == 1 {
		reply, err := exchangeUpstream(ctx, upstreams[0], m)
		if err != nil {
			return nil, "", err
		}
		return reply, upstreams[0].Address(), nil
	}

	raceCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	type result struct {
		reply *dns.Msg
		addr  string
		err   error
	}

	resCh := make(chan result, len(upstreams))
	var wg sync.WaitGroup
	wg.Add(len(upstreams))

	for _, u := range upstreams {
		go func(u stdUpstream) {
			defer wg.Done()
			reply, err := exchangeUpstream(raceCtx, u, m)
			select {
			case resCh <- result{reply: reply, addr: u.Address(), err: err}:
			case <-raceCtx.Done():
				// Context cancelled before we could send, skip this result
			}
		}(u)
	}

	// Helper goroutine to close channel when all senders are done
	go func() {
		wg.Wait()
		close(resCh)
	}()

	var lastErr error
	received := 0
	for received < len(upstreams) {
		select {
		case res, ok := <-resCh:
			if !ok {
				// Channel closed, no more results expected
				break
			}
			received++
			if res.err == nil && res.reply != nil {
				return res.reply, res.addr, nil
			}
			lastErr = res.err
		case <-ctx.Done():
			// Timeout or cancellation; return with whatever error we have
			if lastErr != nil {
				return nil, "", lastErr
			}
			return nil, "", ctx.Err()
		}
	}

	if lastErr != nil {
		return nil, "", lastErr
	}
	return nil, "", fmt.Errorf("all upstreams failed")
}

func (b *stdBackend) Exchange(m *dns.Msg) (*dns.Msg, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), b.timeout)
	defer cancel()
	return b.ExchangeContext(ctx, m)
}

func (b *stdBackend) ExchangeContext(ctx context.Context, m *dns.Msg) (*dns.Msg, string, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && b.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, b.timeout)
		defer cancel()
	}
	reply, addr, err := exchangeParallel(ctx, m, b.upstreams)
	if err != nil {
		return nil, "", err
	}
	return reply, addr, nil
}

func (b *stdBackend) Close() error {
	for _, u := range b.upstreams {
		if c, ok := u.(io.Closer); ok {
			_ = c.Close()
		}
	}
	return nil
}

// nameResolver maps a hostname to an IP literal using bootstrap DNS.
// It is nil when no usable bootstrap servers are configured.
type nameResolver func(host string) (string, error)

func newBackend(cfg *config.Config, rules *rules.Rules, dialer *net.Dialer) dnsBackend {
	timeout := time.Duration(cfg.Timeout.DNS) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	resolve := newBootstrapResolver(cfg.DNS.BootstrapDNS, timeout, dialer)

	var upstreams []stdUpstream
	for _, ns := range cfg.DNS.Nameserver {
		u, err := parseUpstream(ns, timeout, dialer, resolve)
		if err != nil {
			logger.DNS().Warn("dns upstream parse failed", "upstream", ns, "error", err)
			continue
		}
		upstreams = append(upstreams, u)
	}

	if len(upstreams) == 0 {
		return nil
	}

	return &stdBackend{
		upstreams: upstreams,
		timeout:   timeout,
	}
}

// newBootstrapResolver builds a hostname→IP helper from IP-literal bootstrap
// servers. Hostname-based bootstrap entries are skipped: they would need a
// resolver of their own (the chicken-and-egg the setting exists to break).
func newBootstrapResolver(servers []string, timeout time.Duration, dialer *net.Dialer) nameResolver {
	var upstreams []stdUpstream
	for _, ns := range servers {
		if isHostnameEndpoint(ns) {
			logger.DNS().Warn("dns bootstrap skipped (needs an IP literal)", "upstream", ns)
			continue
		}
		u, err := parseUpstream(ns, timeout, dialer, nil)
		if err != nil {
			logger.DNS().Warn("dns bootstrap parse failed", "upstream", ns, "error", err)
			continue
		}
		upstreams = append(upstreams, u)
	}
	if len(upstreams) == 0 {
		return nil
	}

	backend := &stdBackend{upstreams: upstreams, timeout: timeout}
	var mu sync.Mutex
	var sf singleflight.Group
	type cached struct {
		ip      string
		expires time.Time
	}
	cache := map[string]cached{}

	return func(host string) (string, error) {
		if ip := net.ParseIP(host); ip != nil {
			return host, nil
		}
		now := time.Now()
		mu.Lock()
		if e, ok := cache[host]; ok && now.Before(e.expires) {
			ip := e.ip
			mu.Unlock()
			return ip, nil
		}
		mu.Unlock()

		v, err, _ := sf.Do(host, func() (any, error) {
			now := time.Now()
			mu.Lock()
			if e, ok := cache[host]; ok && now.Before(e.expires) {
				ip := e.ip
				mu.Unlock()
				return ip, nil
			}
			mu.Unlock()

			m := new(dns.Msg)
			m.SetQuestion(dns.Fqdn(host), dns.TypeA)
			reply, addr, err := backend.Exchange(m)
			if err != nil {
				return nil, fmt.Errorf("bootstrap resolve %s: %w", host, err)
			}
			var ip string
			ttl := uint32(300)
			for _, ans := range reply.Answer {
				if a, ok := ans.(*dns.A); ok {
					ip = a.A.String()
					ttl = a.Hdr.Ttl
					break
				}
			}
			if ip == "" {
				return nil, fmt.Errorf("bootstrap resolve %s: no A record from %s", host, addr)
			}
			if ttl == 0 {
				ttl = 60
			}
			mu.Lock()
			cache[host] = cached{ip: ip, expires: time.Now().Add(time.Duration(ttl) * time.Second)}
			mu.Unlock()
			logger.DNS().Debug("dns bootstrap resolved", "host", host, "ip", ip, "via", addr)
			return ip, nil
		})
		if err != nil {
			return "", err
		}
		return v.(string), nil
	}
}

func parseUpstream(addr string, timeout time.Duration, dialer *net.Dialer, resolve nameResolver) (stdUpstream, error) {
	if strings.HasPrefix(addr, "https://") {
		// Build the Transport eagerly with the protect dialer wired in. We can't
		// rely on dohUpstream.Exchange's lazy install, because an http.Client
		// constructed with a non-zero Timeout (or any Transport) makes
		// client.Transport != nil and the lazy path never fires — leaving DoH
		// on the default dialer, which on Android gets captured by the VPN's
		// own TUN and deadlocks DNS resolution.
		//
		// DialContext also resolves hostname DoH via bootstrap so the system
		// resolver (127.0.0.1:53 on Android VPN) is never consulted. TLS SNI
		// still uses the original URL host.
		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				return dialResolved(ctx, network, address, dialer, resolve, timeout)
			},
		}
		return &dohUpstream{addr: addr, client: &http.Client{Timeout: timeout, Transport: transport}, dialer: dialer}, nil
	}
	if strings.HasPrefix(addr, "tls://") {
		return &dnsUpstream{
			addr:    ensureHostPort(strings.TrimPrefix(addr, "tls://"), "853"),
			network: "tcp-tls",
			timeout: timeout,
			dialer:  dialer,
			resolve: resolve,
		}, nil
	}
	if strings.HasPrefix(addr, "tcp://") {
		return &dnsUpstream{
			addr:    ensureHostPort(strings.TrimPrefix(addr, "tcp://"), "53"),
			network: "tcp",
			timeout: timeout,
			dialer:  dialer,
			resolve: resolve,
		}, nil
	}
	// Default to UDP
	hostPort := ensureHostPort(strings.TrimPrefix(addr, "udp://"), "53")
	return &dnsUpstream{addr: hostPort, network: "udp", timeout: timeout, dialer: dialer, resolve: resolve}, nil
}

// ensureHostPort appends defaultPort when addr has no port. Bracketed IPv6
// without a port ("[2001:db8::1]") is handled by SplitHostPort failing.
func ensureHostPort(addr, defaultPort string) string {
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}
	if ip := net.ParseIP(strings.Trim(addr, "[]")); ip != nil && ip.To4() == nil {
		return net.JoinHostPort(ip.String(), defaultPort)
	}
	return addr + ":" + defaultPort
}

// isHostnameEndpoint reports whether an http(s)/tls/tcp/udp entry's host is a
// name rather than an IP literal. Used to reject hostname bootstrap servers
// (they cannot resolve themselves).
func isHostnameEndpoint(ns string) bool {
	lower := strings.ToLower(ns)
	rest := ns
	switch {
	case strings.HasPrefix(lower, "https://"):
		rest = ns[len("https://"):]
	case strings.HasPrefix(lower, "http://"):
		rest = ns[len("http://"):]
	case strings.HasPrefix(lower, "tls://"):
		rest = ns[len("tls://"):]
	case strings.HasPrefix(lower, "tcp://"):
		rest = ns[len("tcp://"):]
	case strings.HasPrefix(lower, "udp://"):
		rest = ns[len("udp://"):]
	}
	if i := strings.IndexAny(rest, "/"); i >= 0 {
		rest = rest[:i]
	}
	if host, _, err := net.SplitHostPort(rest); err == nil {
		rest = host
	} else {
		rest = strings.Trim(rest, "[]")
	}
	return net.ParseIP(strings.TrimSpace(rest)) == nil
}

func dialResolved(ctx context.Context, network, address string, dialer *net.Dialer, resolve nameResolver, timeout time.Duration) (net.Conn, error) {
	if resolve != nil {
		host, port, err := net.SplitHostPort(address)
		if err == nil && net.ParseIP(host) == nil {
			ip, rerr := resolve(host)
			if rerr != nil {
				return nil, rerr
			}
			address = net.JoinHostPort(ip, port)
		}
	}
	d := dialer
	if d == nil {
		d = &net.Dialer{Timeout: timeout}
	}
	return d.DialContext(ctx, network, address)
}

const (
	maxDNSConnPoolSize    = 8
	defaultDNSConnIdleTTL = 15 * time.Second
)

type pooledDNSConn struct {
	conn     *dns.Conn
	addr     string
	lastUsed time.Time
}

type dnsUpstream struct {
	addr        string
	network     string
	timeout     time.Duration
	dialer      *net.Dialer // Optional custom dialer (e.g., for VPN socket protection)
	resolve     nameResolver
	tlsName     string        // SNI when addr has been rewritten to an IP
	tlsConfig   *tls.Config   // Optional base TLS config (used by tests for custom RootCAs)
	poolIdleTTL time.Duration // Optional override for testing; defaults to defaultDNSConnIdleTTL

	tlsInitOnce sync.Once
	baseTLS     *tls.Config

	poolMu sync.Mutex
	closed bool
	pool   []pooledDNSConn
}

func (u *dnsUpstream) Address() string { return u.addr }

func (u *dnsUpstream) idleTTL() time.Duration {
	if u.poolIdleTTL > 0 {
		return u.poolIdleTTL
	}
	return defaultDNSConnIdleTTL
}

func (u *dnsUpstream) ensureTLSConfig(serverName string) *tls.Config {
	u.tlsInitOnce.Do(func() {
		sessCache := tls.NewLRUClientSessionCache(64)
		if u.tlsConfig != nil {
			cfg := u.tlsConfig.Clone()
			if cfg.ServerName == "" && serverName != "" {
				cfg.ServerName = serverName
			}
			if cfg.ClientSessionCache == nil {
				cfg.ClientSessionCache = sessCache
			}
			u.baseTLS = cfg
			return
		}
		u.baseTLS = &tls.Config{
			InsecureSkipVerify: false,
			ServerName:         serverName,
			ClientSessionCache: sessCache,
		}
	})
	return u.baseTLS
}

func (u *dnsUpstream) getPooledConn(addr string) (*dns.Conn, bool) {
	now := time.Now()
	ttl := u.idleTTL()
	var stale []*dns.Conn
	var found *dns.Conn

	u.poolMu.Lock()
	for len(u.pool) > 0 {
		idx := len(u.pool) - 1
		pc := u.pool[idx]
		u.pool[idx] = pooledDNSConn{}
		u.pool = u.pool[:idx]

		if pc.addr == addr && now.Sub(pc.lastUsed) <= ttl {
			found = pc.conn
			break
		}
		stale = append(stale, pc.conn)
	}
	u.poolMu.Unlock()

	for _, c := range stale {
		_ = c.Close()
	}
	return found, found != nil
}

func (u *dnsUpstream) putPooledConn(addr string, conn *dns.Conn) {
	if conn == nil {
		return
	}
	u.poolMu.Lock()
	if !u.closed && len(u.pool) < maxDNSConnPoolSize {
		u.pool = append(u.pool, pooledDNSConn{
			conn:     conn,
			addr:     addr,
			lastUsed: time.Now(),
		})
		u.poolMu.Unlock()
		return
	}
	u.poolMu.Unlock()
	_ = conn.Close()
}

func (u *dnsUpstream) Close() error {
	u.poolMu.Lock()
	u.closed = true
	conns := u.pool
	u.pool = nil
	u.poolMu.Unlock()

	for _, pc := range conns {
		if pc.conn != nil {
			_ = pc.conn.Close()
		}
	}
	return nil
}

func (u *dnsUpstream) Exchange(m *dns.Msg) (*dns.Msg, error) {
	timeout := u.timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return u.ExchangeContext(ctx, m)
}

func (u *dnsUpstream) ExchangeContext(ctx context.Context, m *dns.Msg) (*dns.Msg, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	addr := u.addr
	serverName := u.tlsName
	if u.resolve != nil {
		host, port, err := net.SplitHostPort(addr)
		if err == nil && net.ParseIP(host) == nil {
			ip, rerr := u.resolve(host)
			if rerr != nil {
				return nil, rerr
			}
			addr = net.JoinHostPort(ip, port)
			if serverName == "" {
				serverName = host
			}
		}
	}

	client := &dns.Client{
		Net:     u.network,
		Timeout: u.timeout,
	}
	if u.dialer != nil {
		client.Dialer = u.dialer
	}
	if u.network == "tcp-tls" {
		client.TLSConfig = u.ensureTLSConfig(serverName)
	}

	if u.network != "tcp" && u.network != "tcp-tls" {
		reply, _, err := client.ExchangeContext(ctx, m, addr)
		return reply, err
	}

	conn, reused := u.getPooledConn(addr)
	if conn == nil {
		var err error
		conn, err = client.DialContext(ctx, addr)
		if err != nil {
			return nil, err
		}
	}

	reply, _, err := client.ExchangeWithConnContext(ctx, m, conn)
	if err == nil {
		u.putPooledConn(addr, conn)
		return reply, nil
	}

	// Never return an errored connection to the pool (wire framing may be desynchronized).
	_ = conn.Close()

	// Retry once only when a reused pooled connection failed and caller context is still valid.
	if reused && ctx.Err() == nil {
		freshConn, dialErr := client.DialContext(ctx, addr)
		if dialErr != nil {
			return nil, dialErr
		}
		reply, _, err = client.ExchangeWithConnContext(ctx, m, freshConn)
		if err == nil {
			u.putPooledConn(addr, freshConn)
			return reply, nil
		}
		_ = freshConn.Close()
		return nil, err
	}

	return nil, err
}

type dohUpstream struct {
	addr          string
	client        *http.Client
	dialer        *net.Dialer // Optional custom dialer
	transportOnce sync.Once
}

func (u *dohUpstream) Address() string { return u.addr }

func (u *dohUpstream) Exchange(m *dns.Msg) (*dns.Msg, error) {
	ctx := context.Background()
	if u.client != nil && u.client.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, u.client.Timeout)
		defer cancel()
	}
	return u.ExchangeContext(ctx, m)
}

func (u *dohUpstream) ExchangeContext(ctx context.Context, m *dns.Msg) (*dns.Msg, error) {
	if u.dialer != nil {
		u.transportOnce.Do(func() {
			if u.client.Transport == nil {
				u.client.Transport = &http.Transport{
					DialContext: u.dialer.DialContext,
				}
			}
		})
	}
	data, err := m.Pack()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.addr, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("doh status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	reply := new(dns.Msg)
	err = reply.Unpack(body)
	return reply, err
}
