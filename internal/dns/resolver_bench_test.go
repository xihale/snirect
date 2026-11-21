package dns

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	miekgdns "github.com/miekg/dns"
	"github.com/xihale/snirect/internal/config"
)

func BenchmarkResolver_GetCache_Parallel(b *testing.B) {
	r := NewResolver(&config.Config{
		Limit: config.LimitConfig{DNSCacheSize: 10000},
	}, nil)
	defer r.Close()

	const numHosts = 1000
	hosts := make([]string, numHosts)
	for i := 0; i < numHosts; i++ {
		hosts[i] = "host-" + strconv.Itoa(i) + ".example.com"
		r.setCache(hosts[i], "192.0.2.1", miekgdns.TypeA, 300)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			ip, ok := r.getCache(hosts[i%numHosts], miekgdns.TypeA)
			if !ok || ip == "" {
				b.Fatalf("expected cache hit for %s", hosts[i%numHosts])
			}
			i++
		}
	})
}

func BenchmarkResolver_SetCache_Evict(b *testing.B) {
	const cacheCap = 10000
	r := NewResolver(&config.Config{
		Limit: config.LimitConfig{DNSCacheSize: cacheCap},
	}, nil)
	defer r.Close()

	for i := 0; i < cacheCap; i++ {
		r.setCache("seed-"+strconv.Itoa(i)+".example.com", "192.0.2.1", miekgdns.TypeA, 300)
	}

	keys := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = "evict-" + strconv.Itoa(i) + ".example.com"
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.setCache(keys[i], "192.0.2.2", miekgdns.TypeA, 300)
	}
}

type slowMockBackend struct {
	calls atomic.Int64
	delay time.Duration
	ip    net.IP
}

func (m *slowMockBackend) Exchange(q *miekgdns.Msg) (*miekgdns.Msg, string, error) {
	m.calls.Add(1)
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	msg := new(miekgdns.Msg)
	msg.SetReply(q)
	msg.Rcode = miekgdns.RcodeSuccess
	msg.Answer = append(msg.Answer, &miekgdns.A{
		Hdr: miekgdns.RR_Header{
			Name:   q.Question[0].Name,
			Rrtype: miekgdns.TypeA,
			Class:  miekgdns.ClassINET,
			Ttl:    300,
		},
		A: m.ip,
	})
	return msg, "mock-upstream", nil
}

func BenchmarkResolver_Lookup_Singleflight(b *testing.B) {
	backend := &slowMockBackend{
		delay: 200 * time.Microsecond,
		ip:    net.ParseIP("192.0.2.10").To4(),
	}
	r := NewResolver(&config.Config{
		Limit:   config.LimitConfig{DNSCacheSize: 10000},
		Timeout: config.TimeoutConfig{DNS: 5},
	}, nil)
	defer r.Close()
	r.backend = backend

	hosts := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		hosts[i] = "sf-" + strconv.Itoa(i) + ".example.com"
	}

	const concurrency = 16
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		host := hosts[i]
		var wg sync.WaitGroup
		wg.Add(concurrency)
		for g := 0; g < concurrency; g++ {
			go func() {
				defer wg.Done()
				ip, _, err := r.lookupType(ctx, host, miekgdns.TypeA, nil)
				if err != nil || ip != "192.0.2.10" {
					b.Errorf("lookupType failed: ip=%q err=%v", ip, err)
				}
			}()
		}
		wg.Wait()
	}
	b.StopTimer()
	b.ReportMetric(float64(backend.calls.Load())/float64(b.N), "queries/op")
}

var (
	benchDoTCertOnce sync.Once
	benchDoTCert     tls.Certificate
	benchDoTPEMPath  string
	benchDoTCertErr  error
)

func ensureBenchDoTCert() (tls.Certificate, string, error) {
	benchDoTCertOnce.Do(func() {
		priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			benchDoTCertErr = err
			return
		}
		tmpl := &x509.Certificate{
			SerialNumber: big.NewInt(1),
			Subject:      pkix.Name{CommonName: "127.0.0.1"},
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(24 * time.Hour),
			KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
			DNSNames:     []string{"localhost"},
			IsCA:         true,
			BasicConstraintsValid: true,
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
		if err != nil {
			benchDoTCertErr = err
			return
		}
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		keyBytes, err := x509.MarshalECPrivateKey(priv)
		if err != nil {
			benchDoTCertErr = err
			return
		}
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			benchDoTCertErr = err
			return
		}

		dir, err := os.MkdirTemp("", "snirect-dot-bench-*")
		if err != nil {
			benchDoTCertErr = err
			return
		}
		pemPath := filepath.Join(dir, "ca.pem")
		if err := os.WriteFile(pemPath, certPEM, 0600); err != nil {
			benchDoTCertErr = err
			return
		}
		if err := os.Setenv("SSL_CERT_FILE", pemPath); err != nil {
			benchDoTCertErr = err
			return
		}
		benchDoTCert = cert
		benchDoTPEMPath = pemPath
	})
	return benchDoTCert, benchDoTPEMPath, benchDoTCertErr
}

type countingListener struct {
	net.Listener
	accepts atomic.Int64
}

func (l *countingListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err == nil {
		l.accepts.Add(1)
	}
	return c, err
}

func startBenchDoTServer(b *testing.B) (addr string, cl *countingListener, stop func()) {
	b.Helper()
	cert, _, err := ensureBenchDoTCert()
	if err != nil {
		b.Fatalf("ensureBenchDoTCert: %v", err)
	}
	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("listen: %v", err)
	}
	cl = &countingListener{Listener: rawLn}
	tlsLn := tls.NewListener(cl, &tls.Config{
		Certificates: []tls.Certificate{cert},
	})

	respIP := net.ParseIP("192.0.2.55").To4()
	started := make(chan struct{})
	srv := &miekgdns.Server{
		Listener:      tlsLn,
		Net:           "tcp-tls",
		MaxTCPQueries: -1,
		IdleTimeout:   func() time.Duration { return 30 * time.Second },
		NotifyStartedFunc: func() {
			close(started)
		},
		Handler: miekgdns.HandlerFunc(func(w miekgdns.ResponseWriter, r *miekgdns.Msg) {
			m := new(miekgdns.Msg)
			m.SetReply(r)
			m.Answer = append(m.Answer, &miekgdns.A{
				Hdr: miekgdns.RR_Header{
					Name:   r.Question[0].Name,
					Rrtype: miekgdns.TypeA,
					Class:  miekgdns.ClassINET,
					Ttl:    300,
				},
				A: respIP,
			})
			_ = w.WriteMsg(m)
		}),
	}
	go func() {
		_ = srv.ActivateAndServe()
	}()
	<-started
	return rawLn.Addr().String(), cl, func() {
		_ = srv.Shutdown()
	}
}

func BenchmarkDNSUpstream_DoT_Pooled(b *testing.B) {
	addr, cl, stop := startBenchDoTServer(b)
	defer stop()

	u := &dnsUpstream{
		addr:    addr,
		network: "tcp-tls",
		timeout: 5 * time.Second,
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		q := new(miekgdns.Msg)
		q.SetQuestion("example.com.", miekgdns.TypeA)
		q.RecursionDesired = true
		for pb.Next() {
			q.Id = miekgdns.Id()
			reply, err := u.Exchange(q)
			if err != nil || reply == nil || len(reply.Answer) == 0 {
				b.Fatalf("DoT Exchange failed: reply=%v err=%v", reply, err)
			}
		}
	})
	b.StopTimer()
	b.ReportMetric(float64(cl.accepts.Load())/float64(b.N), "conns/op")
}
