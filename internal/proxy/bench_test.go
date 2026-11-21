package proxy

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/xihale/snirect/internal/config"
	"github.com/xihale/snirect/internal/rules"
)

func buildBenchChain(b *testing.B) (leaf, inter, root *x509.Certificate, pool *x509.CertPool) {
	b.Helper()
	now := time.Now()

	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		b.Fatal(err)
	}
	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Bench Root CA"},
		NotBefore:             now.Add(-24 * time.Hour),
		NotAfter:              now.Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)
	if err != nil {
		b.Fatal(err)
	}
	root, err = x509.ParseCertificate(rootDER)
	if err != nil {
		b.Fatal(err)
	}

	interKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		b.Fatal(err)
	}
	interTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "Bench Intermediate CA"},
		NotBefore:             now.Add(-24 * time.Hour),
		NotAfter:              now.Add(30 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	interDER, err := x509.CreateCertificate(rand.Reader, interTmpl, root, &interKey.PublicKey, rootKey)
	if err != nil {
		b.Fatal(err)
	}
	inter, err = x509.ParseCertificate(interDER)
	if err != nil {
		b.Fatal(err)
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		b.Fatal(err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "bench.example.com"},
		NotBefore:    now.Add(-24 * time.Hour),
		NotAfter:     now.Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"bench.example.com"},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, inter, &leafKey.PublicKey, interKey)
	if err != nil {
		b.Fatal(err)
	}
	leaf, err = x509.ParseCertificate(leafDER)
	if err != nil {
		b.Fatal(err)
	}

	pool = x509.NewCertPool()
	pool.AddCert(root)
	return leaf, inter, root, pool
}

func BenchmarkVerifyCert(b *testing.B) {
	leaf, inter, root, pool := buildBenchChain(b)
	policy := rules.CertPolicy{Enabled: true, Strict: true}

	b.Run("WithRootPool", func(b *testing.B) {
		conn := &mockConn{state: buildConnState(leaf, inter, root)}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if !verifyConnCert(conn, "bench.example.com", "", policy, false, pool) {
				b.Fatal("expected verifyConnCert to pass")
			}
		}
	})

	b.Run("SystemRoots", func(b *testing.B) {
		state := buildConnState(leaf, inter)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Exercises the nil-roots (production VerifyCert) chain verification path without WARN log spam.
			_ = verifyCertificateChain(leaf, state, nil)
		}
	})
}

func BenchmarkProxy_Tunnel_Throughput(b *testing.B) {
	const payloadSize = 1 << 20 // 1 MiB per iteration
	payload := bytes.Repeat([]byte("A"), payloadSize)
	sink := make([]byte, 64*1024)

	ps := &ProxyServer{
		Config: &config.Config{
			Server: config.ServerConfig{BufferSize: 65536},
		},
	}

	b.SetBytes(payloadSize)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c1Client, c1Proxy := net.Pipe()
		c2Proxy, c2Server := net.Pipe()

		var wg sync.WaitGroup
		wg.Add(3)
		go func() {
			defer wg.Done()
			ps.tunnel(c1Proxy, c2Proxy)
		}()
		go func() {
			defer wg.Done()
			_, _ = c1Client.Write(payload)
			_ = c1Client.Close()
		}()
		go func() {
			defer wg.Done()
			for {
				_, err := c2Server.Read(sink)
				if err != nil {
					break
				}
			}
			_ = c2Server.Close()
		}()
		wg.Wait()
	}
}

type readerConn struct {
	net.Conn
	r *bytes.Reader
}

func (c *readerConn) Read(p []byte) (int, error) {
	return c.r.Read(p)
}

func buildRealisticClientHello(sni string, targetSize int) []byte {
	var ext bytes.Buffer
	// server_name extension (type 0x0000)
	if sni != "" {
		nameLen := len(sni)
		listLen := 3 + nameLen
		extLen := 2 + listLen
		ext.Write([]byte{0x00, 0x00, byte(extLen >> 8), byte(extLen)})
		ext.Write([]byte{byte(listLen >> 8), byte(listLen), 0x00, byte(nameLen >> 8), byte(nameLen)})
		ext.WriteString(sni)
	}
	// padding extension (type 0x0015) to pad ClientHello to ~targetSize
	baseBodyLen := 2 + 32 + 1 + 32 + 2 + 32 + 1 + 1 + 2 + ext.Len()
	if padLen := targetSize - (5 + 4 + baseBodyLen + 4); padLen > 0 {
		ext.Write([]byte{0x00, 0x15, byte(padLen >> 8), byte(padLen)})
		ext.Write(make([]byte, padLen))
	}

	var body bytes.Buffer
	body.Write([]byte{0x03, 0x03})                 // legacy_version TLS 1.2
	body.Write(make([]byte, 32))                   // random
	body.WriteByte(32)                             // session_id len
	body.Write(make([]byte, 32))                   // session_id
	body.Write([]byte{0x00, 0x20})                 // cipher_suites len (32)
	body.Write(make([]byte, 32))                   // cipher_suites
	body.Write([]byte{0x01, 0x00})                 // compression_methods
	extBytes := ext.Bytes()
	body.Write([]byte{byte(len(extBytes) >> 8), byte(len(extBytes))})
	body.Write(extBytes)

	hsLen := body.Len()
	recLen := 4 + hsLen
	var rec bytes.Buffer
	rec.Write([]byte{22, 0x03, 0x01, byte(recLen >> 8), byte(recLen)})
	rec.Write([]byte{1, byte(hsLen >> 16), byte(hsLen >> 8), byte(hsLen)})
	rec.Write(body.Bytes())
	return rec.Bytes()
}

func BenchmarkPeekClientHelloSNI(b *testing.B) {
	helloBytes := buildRealisticClientHello("example.org", 1536)
	var r bytes.Reader
	rc := &readerConn{r: &r}

	b.SetBytes(int64(len(helloBytes)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Reset(helloBytes)
		replay, sni, err := peekClientHelloSNI(rc)
		if err != nil || sni != "example.org" {
			b.Fatalf("unexpected peek result: sni=%q err=%v", sni, err)
		}
		_, _ = io.Copy(io.Discard, replay)
	}
}

func BenchmarkIsCodeloadBlobPath(b *testing.B) {
	paths := []string{
		"/owner/repo/tar.gz/refs/tags/v1.0",
		"/owner/repo/archive/v1.0/repo-v1.0.tar.gz",
		"/owner/repo/zip/refs/heads/main",
		"/owner/repo",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isCodeloadBlobPath(paths[i&3])
	}
}
