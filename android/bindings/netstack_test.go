package core

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/xihale/snirect/internal/rules"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

func TestRelayToProxy_PreservesBufferedBytes(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	const earlyFlight = "EARLY-SERVER-FLIGHT-BYTES"
	const laterFlight = "-AND-STREAMED-PAYLOAD"
	const clientPayload = "CLIENT-HELLO-BYTES"

	proxyErrCh := make(chan error, 1)
	receivedFromClient := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			proxyErrCh <- err
			return
		}
		defer conn.Close()

		br := bufio.NewReader(conn)
		var hdrs strings.Builder
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				proxyErrCh <- err
				return
			}
			hdrs.WriteString(line)
			if line == "\r\n" {
				break
			}
		}
		if !strings.Contains(hdrs.String(), "CONNECT 1.2.3.4:8443 HTTP/1.1\r\n") {
			proxyErrCh <- io.ErrUnexpectedEOF
			return
		}
		if !strings.Contains(hdrs.String(), "X-Snirect-Src: 10.0.0.2:12345\r\n") {
			proxyErrCh <- io.ErrUnexpectedEOF
			return
		}

		// Write CONNECT response AND early server flight in a single Write so
		// relayToProxy's bufio.Reader buffers earlyFlight past the headers.
		resp := "HTTP/1.1 200 Connection Established\r\nX-Proxy: test\r\n\r\n" + earlyFlight
		if _, err := conn.Write([]byte(resp)); err != nil {
			proxyErrCh <- err
			return
		}
		if _, err := conn.Write([]byte(laterFlight)); err != nil {
			proxyErrCh <- err
			return
		}

		buf := make([]byte, len(clientPayload))
		if _, err := io.ReadFull(br, buf); err != nil {
			proxyErrCh <- err
			return
		}
		receivedFromClient <- string(buf)
		proxyErrCh <- nil
	}()

	clientSide, relayClient := net.Pipe()
	b := &netstackBridge{
		localPort:   port,
		proxyDialer: &net.Dialer{Timeout: 5 * time.Second},
	}
	flow := flowTuple{
		srcIP:   net.IPv4(10, 0, 0, 2).To4(),
		dstIP:   net.IPv4(1, 2, 3, 4).To4(),
		srcPort: 12345,
		dstPort: 8443,
	}

	relayDone := make(chan struct{})
	go func() {
		defer close(relayDone)
		b.relayToProxy(relayClient, flow)
	}()

	if _, err := clientSide.Write([]byte(clientPayload)); err != nil {
		t.Fatalf("client write: %v", err)
	}
	wantServer := earlyFlight + laterFlight
	gotServer, err := io.ReadAll(clientSide)
	if err != nil {
		t.Fatalf("client read: %v", err)
	}
	_ = clientSide.Close()
	<-relayDone

	if err := <-proxyErrCh; err != nil {
		t.Fatalf("mock proxy error: %v", err)
	}
	if gotClient := <-receivedFromClient; gotClient != clientPayload {
		t.Fatalf("proxy received %q, want %q", gotClient, clientPayload)
	}
	if string(gotServer) != wantServer {
		t.Fatalf("client received %q, want %q (port=%s)", string(gotServer), wantServer, strconv.Itoa(port))
	}
}

func TestPacketBytes_MultiSliceMatchesToView(t *testing.T) {
	part1 := []byte{0x45, 0x00, 0x00, 0x3c, 0x12, 0x34, 0x40, 0x00, 0x40, 0x06, 0x00, 0x00, 10, 0, 0, 2, 1, 2, 3, 4}
	part2 := []byte{0x04, 0xd2, 0x01, 0xbb, 0, 0, 0, 1, 0, 0, 0, 0, 0x50, 0x02, 0x20, 0x00, 0, 0, 0, 0}
	part3 := []byte("hello-gvisor-multi-view-payload-chunk-A")
	part4 := []byte("-and-chunk-B-0123456789")

	payloadBuf := buffer.MakeWithData(append([]byte(nil), part3...))
	payloadBuf.Append(buffer.NewViewWithData(part4))
	pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
		ReserveHeaderBytes: len(part1) + len(part2),
		Payload:            payloadBuf,
	})
	defer pkt.DecRef()

	copy(pkt.TransportHeader().Push(len(part2)), part2)
	copy(pkt.NetworkHeader().Push(len(part1)), part1)

	tv := pkt.ToView()
	defer tv.Release()
	want := tv.AsSlice()

	// Normal scratch capacity.
	scratch := make([]byte, 65536+14)
	got, scratchOut := packetBytes(pkt, scratch)
	if !bytes.Equal(got, want) {
		t.Fatalf("multi-slice mismatch: got %x, want %x", got, want)
	}
	if len(scratchOut) != len(scratch) {
		t.Fatalf("unexpected scratch growth: got %d, want %d", len(scratchOut), len(scratch))
	}

	// Undersized scratch grows cleanly without truncating.
	smallScratch := make([]byte, 8)
	gotGrown, grownOut := packetBytes(pkt, smallScratch)
	if !bytes.Equal(gotGrown, want) {
		t.Fatalf("grown multi-slice mismatch: got %x, want %x", gotGrown, want)
	}
	if len(grownOut) < len(want) {
		t.Fatalf("scratch did not grow: len=%d < %d", len(grownOut), len(want))
	}

	// Single-slice packet returns the slice directly without copying into scratch.
	single := stack.NewPacketBuffer(stack.PacketBufferOptions{
		Payload: buffer.MakeWithData([]byte("single-slice-packet")),
	})
	defer single.DecRef()
	gotSingle, _ := packetBytes(single, scratch)
	if string(gotSingle) != "single-slice-packet" {
		t.Fatalf("single-slice got %q", string(gotSingle))
	}
}

type captureDNSConn struct {
	req  []byte
	resp []byte
	read bool
}

func (c *captureDNSConn) Read(p []byte) (int, error) {
	if c.read {
		return 0, io.EOF
	}
	c.read = true
	return copy(p, c.req), nil
}

func (c *captureDNSConn) Write(p []byte) (int, error) {
	c.resp = append([]byte(nil), p...)
	return len(p), nil
}

func (c *captureDNSConn) Close() error                       { return nil }
func (c *captureDNSConn) LocalAddr() net.Addr                { return &net.UDPAddr{} }
func (c *captureDNSConn) RemoteAddr() net.Addr               { return &net.UDPAddr{} }
func (c *captureDNSConn) SetDeadline(t time.Time) error      { return nil }
func (c *captureDNSConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *captureDNSConn) SetWriteDeadline(t time.Time) error { return nil }

func TestHandleDNS_StaticHostOverrideAndAAAA(t *testing.T) {
	r := rules.NewRules()
	r.Hosts["example.com"] = "93.184.216.34"
	r.Init()
	bridge := &netstackBridge{rules: r}

	// TypeA static host override strips trailing dot and constructs direct &dns.A{}.
	qA := new(dns.Msg)
	qA.SetQuestion("example.com.", dns.TypeA)
	wireA, err := qA.Pack()
	if err != nil {
		t.Fatalf("pack A query: %v", err)
	}
	connA := &captureDNSConn{req: wireA}
	bridge.handleDNS(connA)
	respA := new(dns.Msg)
	if err := respA.Unpack(connA.resp); err != nil {
		t.Fatalf("unpack A reply: %v", err)
	}
	if respA.Rcode != dns.RcodeSuccess || len(respA.Answer) != 1 {
		t.Fatalf("A reply unexpected: rcode=%d answers=%d", respA.Rcode, len(respA.Answer))
	}
	aRec, ok := respA.Answer[0].(*dns.A)
	if !ok || !aRec.A.Equal(net.IPv4(93, 184, 216, 34)) || aRec.Hdr.Ttl != 60 || aRec.Hdr.Name != "example.com." {
		t.Fatalf("unexpected A record: %+v", respA.Answer[0])
	}

	// TypeAAAA when IPv6 disabled returns empty NOERROR.
	qAAAA := new(dns.Msg)
	qAAAA.SetQuestion("example.com.", dns.TypeAAAA)
	wireAAAA, err := qAAAA.Pack()
	if err != nil {
		t.Fatalf("pack AAAA query: %v", err)
	}
	connAAAA := &captureDNSConn{req: wireAAAA}
	bridge.handleDNS(connAAAA)
	respAAAA := new(dns.Msg)
	if err := respAAAA.Unpack(connAAAA.resp); err != nil {
		t.Fatalf("unpack AAAA reply: %v", err)
	}
	if respAAAA.Rcode != dns.RcodeSuccess || len(respAAAA.Answer) != 0 {
		t.Fatalf("AAAA reply unexpected: rcode=%d answers=%d", respAAAA.Rcode, len(respAAAA.Answer))
	}
}

func BenchmarkNetstack_Pipe_CopyBuffer(b *testing.B) {
	const payloadSize = 256 * 1024
	const overread = 64
	header := []byte("HTTP/1.1 200 OK\r\n\r\n")
	payload := make([]byte, payloadSize)
	for i := range payload {
		payload[i] = byte(i)
	}

	b.SetBytes(int64(payloadSize))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		clientSide, client := net.Pipe()
		upstream, upstreamSide := net.Pipe()

		var wg sync.WaitGroup
		wg.Add(2)

		// Simulated proxy writing CONNECT response + over-read bytes + payload.
		go func() {
			defer wg.Done()
			defer upstreamSide.Close()
			_, _ = upstreamSide.Write(header)
			_, _ = upstreamSide.Write(payload[:overread])
			_, _ = upstreamSide.Write(payload[overread:])
		}()

		// Simulated client draining everything sent through pipe.
		go func() {
			defer wg.Done()
			defer clientSide.Close()
			_, _ = io.Copy(io.Discard, clientSide)
		}()

		br := bufio.NewReader(upstream)
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				b.Fatalf("read response: %v", err)
			}
			if line == "\r\n" {
				break
			}
		}

		pipe(client, br, upstream, "bench")
		_ = client.Close()
		_ = upstream.Close()
		wg.Wait()
	}
}

type benchDNSConn struct {
	req  []byte
	read bool
}

func (c *benchDNSConn) Read(p []byte) (int, error) {
	if c.read {
		return 0, io.EOF
	}
	c.read = true
	n := copy(p, c.req)
	return n, nil
}

func (c *benchDNSConn) Write(p []byte) (int, error)        { return len(p), nil }
func (c *benchDNSConn) Close() error                       { return nil }
func (c *benchDNSConn) LocalAddr() net.Addr                { return &net.UDPAddr{} }
func (c *benchDNSConn) RemoteAddr() net.Addr               { return &net.UDPAddr{} }
func (c *benchDNSConn) SetDeadline(t time.Time) error      { return nil }
func (c *benchDNSConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *benchDNSConn) SetWriteDeadline(t time.Time) error { return nil }

func BenchmarkHandleDNS_StaticHostOverride(b *testing.B) {
	r := rules.NewRules()
	r.Hosts["example.com"] = "93.184.216.34"
	r.Init()

	q := new(dns.Msg)
	q.SetQuestion("example.com.", dns.TypeA)
	reqWire, err := q.Pack()
	if err != nil {
		b.Fatalf("pack query: %v", err)
	}

	bridge := &netstackBridge{rules: r}
	conn := &benchDNSConn{req: reqWire}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn.read = false
		bridge.handleDNS(conn)
	}
}
