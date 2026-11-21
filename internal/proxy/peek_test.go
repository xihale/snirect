package proxy

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"
)

// testCert mints a throwaway self-signed pair for handshake tests.
func testCert(t *testing.T) tls.Certificate {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "peek-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: priv}
}

// TestPeekClientHelloSNI_AndReplay drives a real crypto/tls client into a
// listener that peeks the ClientHello: the SNI must be extracted, and the
// replayed stream must still carry a complete handshake for tls.Server.
func TestPeekClientHelloSNI_AndReplay(t *testing.T) {
	cert := testCert(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverDone := make(chan error, 1)
	go func() {
		raw, err := ln.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		replay, sni, perr := peekClientHelloSNI(raw)
		if perr != nil {
			serverDone <- perr
			return
		}
		if sni != "example.org" {
			serverDone <- errUnexpected("sni: " + sni)
			return
		}
		// The replayed conn must satisfy a real server handshake — this is
		// what proves the peeked bytes are handed back intact.
		tc := tls.Server(replay, &tls.Config{
			Certificates: []tls.Certificate{cert},
		})
		if err := tc.Handshake(); err != nil {
			serverDone <- err
			return
		}
		line, err := bufio.NewReader(tc).ReadString('\n')
		if err != nil {
			serverDone <- err
			return
		}
		if line != "hello through replay\n" {
			serverDone <- errUnexpected("payload: " + line)
			return
		}
		serverDone <- nil
	}()

	conn, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{
		ServerName:         "example.org",
		InsecureSkipVerify: true,
	})
	if err != nil {
		t.Fatalf("client dial: %v", err)
	}
	if _, err := conn.Write([]byte("hello through replay\n")); err != nil {
		t.Fatalf("client write: %v", err)
	}
	conn.Close()

	if err := <-serverDone; err != nil {
		t.Fatalf("server side: %v", err)
	}
}

// TestPeekClientHelloSNI_NoSNI covers the SNI-less ClientHello: sni comes
// back empty, peek reports success.
func TestPeekClientHelloSNI_NoSNI(t *testing.T) {
	cert := testCert(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	type result struct {
		sni string
		err error
	}
	done := make(chan result, 1)
	go func() {
		raw, err := ln.Accept()
		if err != nil {
			done <- result{err: err}
			return
		}
		replay, sni, perr := peekClientHelloSNI(raw)
		if perr == nil {
			// Prove replay integrity on the SNI-less path too.
			tc := tls.Server(replay, &tls.Config{Certificates: []tls.Certificate{cert}})
			if hsErr := tc.Handshake(); hsErr != nil {
				perr = hsErr
			} else {
				tc.Close()
			}
		}
		raw.Close()
		done <- result{sni: sni, err: perr}
	}()

	// tls.Dial to a loopback literal with no ServerName sends no SNI.
	conn, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		t.Fatalf("client dial: %v", err)
	}
	conn.Close()

	res := <-done
	if res.err != nil {
		t.Fatalf("peek/handshake: %v", res.err)
	}
	if res.sni != "" {
		t.Fatalf("expected empty SNI, got %q", res.sni)
	}
}

// TestPeekClientHelloSNI_NotTLS verifies non-TLS first bytes fail the peek
// (and would route to a raw tunnel with the bytes replayed).
func TestPeekClientHelloSNI_NotTLS(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()

	go func() {
		_, _ = c2.Write([]byte("GET /plain HTTP/1.1\r\nHost: x\r\n\r\n"))
	}()

	replay, _, err := peekClientHelloSNI(c1)
	if err == nil {
		t.Fatal("expected peek error for non-TLS input")
	}
	// The replay must hand the consumed bytes back.
	buf := make([]byte, 32)
	n, rerr := replay.Read(buf)
	if rerr != nil || string(buf[:n]) != "GET /" {
		t.Fatalf("replay lost bytes: n=%d err=%v %q", n, rerr, buf[:n])
	}
}

type stringErr string

func (e stringErr) Error() string { return string(e) }

func errUnexpected(got string) error { return stringErr("unexpected: " + got) }

func TestParseSNI_MalformedAndBoundary(t *testing.T) {
	// Must never panic on truncated, oversized-length, or garbage inputs.
	cases := [][]byte{
		nil,
		{},
		make([]byte, 34),
		make([]byte, 35), // sidLen=0, missing csLen
		// sidLen claims 200 bytes when buffer ends
		append(make([]byte, 34), 200, 1, 2, 3),
		// sidLen=0, csLen claims 500 bytes
		append(make([]byte, 34), 0, 0x01, 0xf4, 0, 0),
		// sidLen=0, csLen=0, compLen claims 99 bytes
		append(make([]byte, 34), 0, 0, 0, 99, 0),
		// sidLen=0, csLen=0, compLen=0, extLen claims 1000 bytes
		append(make([]byte, 34), 0, 0, 0, 0, 0x03, 0xe8, 0, 0),
		// valid header up to server_name extension with truncated listLen / nameLen
		append(make([]byte, 34), 0, 0, 0, 0, 0, 6, 0, 0, 0, 2, 0, 10),
		append(make([]byte, 34), 0, 0, 0, 0, 0, 9, 0, 0, 0, 5, 0, 3, 0, 0, 50),
	}
	for i, tc := range cases {
		if got := parseSNI(tc); got != "" {
			t.Errorf("case %d: expected empty SNI, got %q", i, got)
		}
	}

	// Valid ClientHello body truncated at every possible byte boundary must not panic.
	full := buildRealisticClientHello("sub.example.org", 512)
	body := full[9:] // skip 5-byte record hdr + 4-byte handshake hdr
	for i := 0; i < len(body); i++ {
		_ = parseSNI(body[:i])
	}
	if got := parseSNI(body); got != "sub.example.org" {
		t.Fatalf("full body parseSNI = %q, want sub.example.org", got)
	}
}

func FuzzParseSNI(f *testing.F) {
	f.Add(buildRealisticClientHello("example.org", 256)[9:])
	f.Add([]byte{})
	f.Add(append(make([]byte, 34), 255))
	f.Fuzz(func(t *testing.T, data []byte) {
		_ = parseSNI(data)
	})
}
