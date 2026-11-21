package dns

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	miekgdns "github.com/miekg/dns"
	"github.com/xihale/snirect/internal/config"
)

func makeSelfSignedTLSConfig(t *testing.T) (*tls.Config, *x509.CertPool) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	srvCfg := &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{der},
			PrivateKey:  priv,
		}},
	}
	return srvCfg, pool
}

func TestDNSUpstream_DoT_StaleConnRetryOnce(t *testing.T) {
	srvTLS, rootPool := makeSelfSignedTLSConfig(t)
	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer rawLn.Close()

	var connNum atomic.Int32
	var queryNum atomic.Int32

	go func() {
		for {
			c, err := rawLn.Accept()
			if err != nil {
				return
			}
			id := connNum.Add(1)
			go func(raw net.Conn, connID int32) {
				tc := tls.Server(raw, srvTLS)
				if err := tc.Handshake(); err != nil {
					_ = tc.Close()
					return
				}
				dc := &miekgdns.Conn{Conn: tc}
				for {
					req, err := dc.ReadMsg()
					if err != nil {
						_ = dc.Close()
						return
					}
					q := queryNum.Add(1)
					resp := new(miekgdns.Msg)
					resp.SetReply(req)
					resp.Answer = append(resp.Answer, &miekgdns.A{
						Hdr: miekgdns.RR_Header{
							Name:   req.Question[0].Name,
							Rrtype: miekgdns.TypeA,
							Class:  miekgdns.ClassINET,
							Ttl:    60,
						},
						A: net.ParseIP("192.0.2.42").To4(),
					})
					if err := dc.WriteMsg(resp); err != nil {
						_ = dc.Close()
						return
					}
					// Force connection 1 closed right after its first reply so the
					// next pooled checkout hits a stale EOF/broken conn.
					if connID == 1 && q == 1 {
						_ = dc.Close()
						return
					}
				}
			}(c, id)
		}
	}()

	u := &dnsUpstream{
		addr:      rawLn.Addr().String(),
		network:   "tcp-tls",
		timeout:   2 * time.Second,
		tlsConfig: &tls.Config{RootCAs: rootPool},
	}
	defer u.Close()

	q1 := new(miekgdns.Msg)
	q1.SetQuestion("one.example.com.", miekgdns.TypeA)
	q1.Id = miekgdns.Id()
	r1, err := u.Exchange(q1)
	if err != nil || len(r1.Answer) != 1 {
		t.Fatalf("first query failed: r1=%v err=%v", r1, err)
	}

	// Give the server goroutine a moment to close conn 1 after writing reply 1.
	time.Sleep(20 * time.Millisecond)

	q2 := new(miekgdns.Msg)
	q2.SetQuestion("two.example.com.", miekgdns.TypeA)
	q2.Id = miekgdns.Id()
	r2, err := u.Exchange(q2)
	if err != nil || len(r2.Answer) != 1 {
		t.Fatalf("second query (stale pooled retry) failed: r2=%v err=%v", r2, err)
	}

	// Third query should reuse conn 2 without dialing conn 3.
	q3 := new(miekgdns.Msg)
	q3.SetQuestion("three.example.com.", miekgdns.TypeA)
	q3.Id = miekgdns.Id()
	r3, err := u.Exchange(q3)
	if err != nil || len(r3.Answer) != 1 {
		t.Fatalf("third query (reused conn 2) failed: r3=%v err=%v", r3, err)
	}

	if got := connNum.Load(); got != 2 {
		t.Fatalf("expected 2 connections total (conn 1 stale + conn 2 reused), got %d", got)
	}
	u.poolMu.Lock()
	poolLen := len(u.pool)
	u.poolMu.Unlock()
	if poolLen != 1 {
		t.Fatalf("expected 1 healthy conn in pool, got %d", poolLen)
	}
}

func TestDNSUpstream_NoRetryOnCancelledContext(t *testing.T) {
	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer rawLn.Close()

	var connNum atomic.Int32
	go func() {
		for {
			c, err := rawLn.Accept()
			if err != nil {
				return
			}
			id := connNum.Add(1)
			go func(raw net.Conn, connID int32) {
				dc := &miekgdns.Conn{Conn: raw}
				req, err := dc.ReadMsg()
				if err != nil {
					_ = dc.Close()
					return
				}
				resp := new(miekgdns.Msg)
				resp.SetReply(req)
				_ = dc.WriteMsg(resp)
				if connID == 1 {
					_ = dc.Close()
				}
			}(c, id)
		}
	}()

	u := &dnsUpstream{
		addr:    rawLn.Addr().String(),
		network: "tcp",
		timeout: 2 * time.Second,
	}
	defer u.Close()

	q1 := new(miekgdns.Msg)
	q1.SetQuestion("ctx.example.com.", miekgdns.TypeA)
	q1.Id = miekgdns.Id()
	if _, err := u.ExchangeContext(context.Background(), q1); err != nil {
		t.Fatalf("warmup query failed: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	q2 := new(miekgdns.Msg)
	q2.SetQuestion("ctx2.example.com.", miekgdns.TypeA)
	q2.Id = miekgdns.Id()
	if _, err := u.ExchangeContext(ctx, q2); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if got := connNum.Load(); got != 1 {
		t.Fatalf("expected no retry dial when ctx cancelled, got %d conns", got)
	}
}

func TestDNSUpstream_IdleTTLEviction(t *testing.T) {
	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer rawLn.Close()

	var connNum atomic.Int32
	go func() {
		for {
			c, err := rawLn.Accept()
			if err != nil {
				return
			}
			connNum.Add(1)
			go func(raw net.Conn) {
				dc := &miekgdns.Conn{Conn: raw}
				defer dc.Close()
				for {
					req, err := dc.ReadMsg()
					if err != nil {
						return
					}
					resp := new(miekgdns.Msg)
					resp.SetReply(req)
					_ = dc.WriteMsg(resp)
				}
			}(c)
		}
	}()

	u := &dnsUpstream{
		addr:        rawLn.Addr().String(),
		network:     "tcp",
		timeout:     2 * time.Second,
		poolIdleTTL: 25 * time.Millisecond,
	}
	defer u.Close()

	q := new(miekgdns.Msg)
	q.SetQuestion("ttl.example.com.", miekgdns.TypeA)
	q.Id = miekgdns.Id()
	if _, err := u.Exchange(q); err != nil {
		t.Fatalf("first query failed: %v", err)
	}

	time.Sleep(40 * time.Millisecond)

	q.Id = miekgdns.Id()
	if _, err := u.Exchange(q); err != nil {
		t.Fatalf("second query after idle TTL failed: %v", err)
	}
	if got := connNum.Load(); got != 2 {
		t.Fatalf("expected fresh conn after idle TTL expiry, got %d conns", got)
	}
}

func TestDNSUpstream_PoolBounded(t *testing.T) {
	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer rawLn.Close()

	go func() {
		for {
			c, err := rawLn.Accept()
			if err != nil {
				return
			}
			go func(raw net.Conn) {
				dc := &miekgdns.Conn{Conn: raw}
				defer dc.Close()
				for {
					req, err := dc.ReadMsg()
					if err != nil {
						return
					}
					time.Sleep(15 * time.Millisecond)
					resp := new(miekgdns.Msg)
					resp.SetReply(req)
					_ = dc.WriteMsg(resp)
				}
			}(c)
		}
	}()

	u := &dnsUpstream{
		addr:    rawLn.Addr().String(),
		network: "tcp",
		timeout: 2 * time.Second,
	}
	defer u.Close()

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q := new(miekgdns.Msg)
			q.SetQuestion("bound.example.com.", miekgdns.TypeA)
			q.Id = miekgdns.Id()
			if _, err := u.Exchange(q); err != nil {
				t.Errorf("concurrent exchange failed: %v", err)
			}
		}()
	}
	wg.Wait()

	u.poolMu.Lock()
	poolLen := len(u.pool)
	u.poolMu.Unlock()
	if poolLen > maxDNSConnPoolSize {
		t.Fatalf("pool exceeded maxDNSConnPoolSize (%d): got %d", maxDNSConnPoolSize, poolLen)
	}
}

type gatedBackend struct {
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (g *gatedBackend) Exchange(m *miekgdns.Msg) (*miekgdns.Msg, string, error) {
	return g.ExchangeContext(context.Background(), m)
}

func (g *gatedBackend) ExchangeContext(ctx context.Context, m *miekgdns.Msg) (*miekgdns.Msg, string, error) {
	g.calls.Add(1)
	g.once.Do(func() { close(g.started) })
	select {
	case <-g.release:
		return makeDNSResponse(miekgdns.TypeA, []string{"192.0.2.99"}, 120), "gated", nil
	case <-ctx.Done():
		return nil, "", ctx.Err()
	}
}

func TestResolver_Lookup_Singleflight_CancelIsolation(t *testing.T) {
	backend := &gatedBackend{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	r := NewResolver(&config.Config{
		Timeout: config.TimeoutConfig{DNS: 5},
	}, nil)
	defer r.Close()
	r.backend = backend

	ctxA, cancelA := context.WithCancel(context.Background())
	errChA := make(chan error, 1)
	go func() {
		_, _, err := r.lookupType(ctxA, "sf-cancel.example.com", miekgdns.TypeA, nil)
		errChA <- err
	}()

	<-backend.started

	type resB struct {
		ip  string
		err error
	}
	resChB := make(chan resB, 1)
	go func() {
		ip, _, err := r.lookupType(context.Background(), "sf-cancel.example.com", miekgdns.TypeA, nil)
		resChB <- resB{ip: ip, err: err}
	}()

	// Let caller B enter DoChan wait before cancelling caller A.
	time.Sleep(15 * time.Millisecond)
	cancelA()

	if err := <-errChA; !errors.Is(err, context.Canceled) {
		t.Fatalf("caller A expected context.Canceled, got %v", err)
	}

	close(backend.release)

	gotB := <-resChB
	if gotB.err != nil || gotB.ip != "192.0.2.99" {
		t.Fatalf("caller B expected 192.0.2.99 with nil error, got ip=%q err=%v", gotB.ip, gotB.err)
	}
	if calls := backend.calls.Load(); calls != 1 {
		t.Fatalf("expected singleflight to coalesce to 1 backend call, got %d", calls)
	}
}

func TestDoHUpstream_ExchangeContext_Cancellation(t *testing.T) {
	reqStarted := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		close(reqStarted)
		<-r.Context().Done()
	}))
	defer srv.Close()

	u := &dohUpstream{
		addr:   srv.URL,
		client: &http.Client{Timeout: 5 * time.Second},
	}
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		q := new(miekgdns.Msg)
		q.SetQuestion("doh-cancel.example.com.", miekgdns.TypeA)
		_, err := u.ExchangeContext(ctx, q)
		errCh <- err
	}()

	<-reqStarted
	cancel()

	err := <-errCh
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled from DoH ExchangeContext, got %v", err)
	}
}

func TestBootstrapResolver_Singleflight(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	var queries atomic.Int32
	started := make(chan struct{})
	srv := &miekgdns.Server{
		PacketConn: pc,
		NotifyStartedFunc: func() {
			close(started)
		},
		Handler: miekgdns.HandlerFunc(func(w miekgdns.ResponseWriter, r *miekgdns.Msg) {
			queries.Add(1)
			time.Sleep(30 * time.Millisecond)
			m := new(miekgdns.Msg)
			m.SetReply(r)
			m.Answer = append(m.Answer, &miekgdns.A{
				Hdr: miekgdns.RR_Header{
					Name:   r.Question[0].Name,
					Rrtype: miekgdns.TypeA,
					Class:  miekgdns.ClassINET,
					Ttl:    120,
				},
				A: net.ParseIP("198.51.100.7").To4(),
			})
			_ = w.WriteMsg(m)
		}),
	}
	go func() { _ = srv.ActivateAndServe() }()
	<-started
	defer srv.Shutdown()

	resolve := newBootstrapResolver([]string{"udp://" + pc.LocalAddr().String()}, 2*time.Second, nil)
	if resolve == nil {
		t.Fatal("expected non-nil bootstrap resolver")
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ip, err := resolve("boot.example.com")
			if err != nil || ip != "198.51.100.7" {
				t.Errorf("bootstrap resolve failed: ip=%q err=%v", ip, err)
			}
		}()
	}
	wg.Wait()

	if got := queries.Load(); got != 1 {
		t.Fatalf("expected 1 coalesced bootstrap query, got %d", got)
	}
}

type countingMockBackend struct {
	mu        sync.Mutex
	aaaaCount int
	aCount    int
	aaaaResp  func() *miekgdns.Msg
	aResp     func() *miekgdns.Msg
}

func (b *countingMockBackend) Exchange(q *miekgdns.Msg) (*miekgdns.Msg, string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch q.Question[0].Qtype {
	case miekgdns.TypeAAAA:
		b.aaaaCount++
		return b.aaaaResp(), "mock", nil
	case miekgdns.TypeA:
		b.aCount++
		return b.aResp(), "mock", nil
	default:
		return nil, "", errors.New("unexpected qtype")
	}
}

func TestResolver_NegativeCache_NODATA_And_TTL(t *testing.T) {
	backend := &countingMockBackend{
		aaaaResp: func() *miekgdns.Msg {
			// RcodeSuccess with 0 AAAA records (NODATA)
			return makeDNSResponse(miekgdns.TypeAAAA, nil, 0)
		},
		aResp: func() *miekgdns.Msg {
			return makeDNSResponse(miekgdns.TypeA, []string{"192.0.2.10"}, 300)
		},
	}

	r := NewResolver(&config.Config{
		IPv6:    true,
		Timeout: config.TimeoutConfig{DNS: 5},
	}, nil)
	defer r.Close()
	r.backend = backend
	r.negativeCacheTTL = 50 * time.Millisecond

	for i := 0; i < 3; i++ {
		ip, err := r.Resolve(context.Background(), "ipv4only.example.com", nil)
		if err != nil || ip != "192.0.2.10" {
			t.Fatalf("Resolve %d failed: ip=%q err=%v", i, ip, err)
		}
	}

	backend.mu.Lock()
	if backend.aaaaCount != 1 || backend.aCount != 1 {
		t.Fatalf("expected 1 AAAA and 1 A query with negative cache, got AAAA=%d A=%d", backend.aaaaCount, backend.aCount)
	}
	backend.mu.Unlock()

	// Invalidate clears negative cache too.
	r.Invalidate("ipv4only.example.com")
	if _, err := r.Resolve(context.Background(), "ipv4only.example.com", nil); err != nil {
		t.Fatalf("Resolve after Invalidate failed: %v", err)
	}
	backend.mu.Lock()
	if backend.aaaaCount != 2 || backend.aCount != 2 {
		t.Fatalf("expected 2 AAAA and 2 A queries after Invalidate, got AAAA=%d A=%d", backend.aaaaCount, backend.aCount)
	}
	backend.mu.Unlock()

	// Wait for negative TTL expiration; AAAA should be re-queried while A remains cached.
	time.Sleep(70 * time.Millisecond)
	if _, err := r.Resolve(context.Background(), "ipv4only.example.com", nil); err != nil {
		t.Fatalf("Resolve after negative TTL failed: %v", err)
	}
	backend.mu.Lock()
	if backend.aaaaCount != 3 || backend.aCount != 2 {
		t.Fatalf("expected 3 AAAA and 2 A queries after negative TTL expiry, got AAAA=%d A=%d", backend.aaaaCount, backend.aCount)
	}
	backend.mu.Unlock()
}

func TestResolver_NegativeCache_NXDOMAIN_vs_SERVFAIL(t *testing.T) {
	var rcode atomic.Int32
	rcode.Store(miekgdns.RcodeNameError)

	backend := &countingMockBackend{
		aResp: func() *miekgdns.Msg {
			m := new(miekgdns.Msg)
			m.Rcode = int(rcode.Load())
			return m
		},
	}
	r := NewResolver(&config.Config{
		Timeout: config.TimeoutConfig{DNS: 5},
	}, nil)
	defer r.Close()
	r.backend = backend

	// NXDOMAIN is negative-cached.
	_, _, err1 := r.lookupType(context.Background(), "nx.example.com", miekgdns.TypeA, nil)
	_, _, err2 := r.lookupType(context.Background(), "nx.example.com", miekgdns.TypeA, nil)
	if err1 == nil || err2 == nil {
		t.Fatalf("expected errors on NXDOMAIN, got %v / %v", err1, err2)
	}
	backend.mu.Lock()
	if backend.aCount != 1 {
		t.Fatalf("expected NXDOMAIN to be cached (1 query), got %d", backend.aCount)
	}
	backend.mu.Unlock()

	// SERVFAIL must NOT be negative-cached.
	rcode.Store(miekgdns.RcodeServerFailure)
	_, _, _ = r.lookupType(context.Background(), "sf.example.com", miekgdns.TypeA, nil)
	_, _, _ = r.lookupType(context.Background(), "sf.example.com", miekgdns.TypeA, nil)
	backend.mu.Lock()
	if backend.aCount != 3 {
		t.Fatalf("expected SERVFAIL not to be negative-cached (2 more queries, total 3), got %d", backend.aCount)
	}
	backend.mu.Unlock()
}

func TestResolver_ConcurrentCacheEvictionAndInvalidate(t *testing.T) {
	const cacheCap = 32
	backend := &mockBackend{
		aResp: makeDNSResponse(miekgdns.TypeA, []string{"192.0.2.1"}, 300),
	}
	r := NewResolver(&config.Config{
		Limit:   config.LimitConfig{DNSCacheSize: cacheCap},
		Timeout: config.TimeoutConfig{DNS: 5},
	}, nil)
	defer r.Close()
	r.backend = backend

	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 150; i++ {
				host := "h-" + strconv.Itoa((worker*37+i)%128) + ".example.com"
				switch i % 4 {
				case 0:
					_, _ = r.Resolve(context.Background(), host, nil)
				case 1:
					r.setCache(host, "192.0.2.2", miekgdns.TypeA, 300)
				case 2:
					_, _ = r.getCache(host, miekgdns.TypeA)
				case 3:
					r.Invalidate(host)
				}
			}
		}(g)
	}
	wg.Wait()

	r.cacheMu.RLock()
	size := len(r.cache)
	r.cacheMu.RUnlock()
	if size > cacheCap {
		t.Fatalf("cache size %d exceeded cap %d", size, cacheCap)
	}
}

func TestResolver_StaticECS_PreParsed(t *testing.T) {
	r := NewResolver(&config.Config{
		ECS: "198.51.100.0/24",
	}, nil)
	defer r.Close()

	if r.staticECSNet == nil {
		t.Fatal("expected staticECSNet to be pre-parsed in NewResolver")
	}
	m := r.buildMessage("ecs.example.com", miekgdns.TypeA, nil)
	opt := m.IsEdns0()
	if opt == nil || len(opt.Option) == 0 {
		t.Fatal("expected EDNS0 ECS option on built message")
	}
	sub, ok := opt.Option[0].(*miekgdns.EDNS0_SUBNET)
	if !ok {
		t.Fatalf("expected *EDNS0_SUBNET, got %T", opt.Option[0])
	}
	if sub.Family != 1 || sub.SourceNetmask != 24 || !sub.Address.Equal(net.ParseIP("198.51.100.0")) {
		t.Fatalf("unexpected ECS option: %+v", sub)
	}
}

func TestDNSUpstream_PutPooledConnAfterClose(t *testing.T) {
	u := &dnsUpstream{addr: "127.0.0.1:53", network: "tcp"}
	if err := u.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	c1, c2 := net.Pipe()
	defer c2.Close()
	dc := &miekgdns.Conn{Conn: c1}

	u.putPooledConn("127.0.0.1:53", dc)

	u.poolMu.Lock()
	poolLen := len(u.pool)
	u.poolMu.Unlock()
	if poolLen != 0 {
		t.Fatalf("expected closed upstream pool to stay empty, got %d", poolLen)
	}

	_ = c2.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 1)
	if _, err := c2.Read(buf); err == nil {
		t.Fatal("expected connection returned after Close to be closed")
	}
}
