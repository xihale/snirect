package core

// This file is the only mobile-specific networking code. It is a deliberately
// thin tun2socks bridge: a gVisor netstack that terminates the TUN FD's IP
// flows and relays every TCP stream into the local proxy.ProxyServer
// (127.0.0.1:<port>), where the real SNI-rewrite + MITM + cert-verify pipeline
// runs — identical to desktop.
//
// Why not MITM inside the netstack (the old core/tun design)?
//   core deleted tun/ entirely (HTTP-proxy mode is now the sole capture path).
//   Re-implementing MITM here would fork the network stack away from core,
//   which violates the workspace boundary rule. Relaying to the in-process
//   proxy keeps one source of truth for the SNI/cert path.
//
// CONNECT synthesis (audit B1): TUN-captured apps speak raw TCP to the original
// destination — they never issue an HTTP CONNECT, so a blind byte pipe into the
// proxy's http.Server could never work (the TLS ClientHello was parsed as a
// request line and 400'd). The relay therefore captures the original
// destination from the forwarder request (r.ID().LocalAddress/LocalPort),
// dials the local proxy, and opens the flow with a synthesized
// "CONNECT <ip>:<port> HTTP/1.1". From there the app's bytes flow through the
// proxy's handleConnect exactly like a desktop browser's CONNECT — SNI is read
// from the ClientHello, so hostname rules keep working untouched.
//
// DNS: captured :53 UDP packets are forwarded to the shared resolver's backend
// (which already dials upstream over the protect() bypass dialer), so
// TUN-captured apps get unpolluted resolution. Failures answer SERVFAIL so
// clients retry/fallback instead of hanging on a black hole (audit B2). This
// mirrors the deleted core/tun/dns.go behavior.
//
// Fatal errors: a TUN read/write error means the engine is dead (interface
// torn down, oversized packet against a smaller VpnService MTU, ...). The
// bridge records the reason and closes itself; Run() then returns a non-nil
// error, which the engine goroutine turns into OnEngineError + full teardown
// (audit B3 — the old code returned nil and the UI kept showing ACTIVE).

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	coreconfig "github.com/xihale/snirect/internal/config"
	coredns "github.com/xihale/snirect/internal/dns"
	"github.com/xihale/snirect/internal/rules"

	"github.com/miekg/dns"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

// tunFile is the minimal *os.File-like handle the bridge reads/writes. It is an
// interface so the off-Android build stub (platform_other.go) can satisfy it
// without touching syscall.OpenFile.
type tunFile interface {
	io.Reader
	io.Writer
	io.Closer
}

// netstackBridge pumps packets between a TUN file and a gVisor stack, relaying
// TCP flows into the local proxy via synthesized CONNECT requests and
// answering DNS via the shared resolver.
type netstackBridge struct {
	f         tunFile
	localPort int // the proxy.ProxyServer's listening port
	resolver  *coredns.Resolver
	rules     *rules.Rules
	cfg       *coreconfig.Config

	stk         *stack.Stack
	ep          *channel.Endpoint
	proxyDialer *net.Dialer // dials 127.0.0.1:localPort (loopback, NOT protect-wrapped)

	mu sync.Mutex
	// closed is set by Close (external stop or our own fatal path);
	// fatalReason is set only when the bridge died on its own, making Run
	// return a non-nil error so the engine goroutine can report engine death.
	closed      bool
	fatalReason string
	done        chan struct{}
}

// newNetstackBridge constructs the gVisor stack bound to 10.0.0.2/24 — the same
// /24 as Android's VpnService (which assigns the kernel 10.0.0.1/24). gVisor
// refuses packets addressed outside its NIC's subnet, so the addresses must
// match. mtu comes from the mobile Config (Kotlin sets it from the VpnService
// builder); zero or negative falls back to 1500. proxyDialer is used only for
// the loopback hop to the local proxy.
func newNetstackBridge(f tunFile, localPort int, mtu int, cfg *coreconfig.Config, resolver *coredns.Resolver, ruleSet *rules.Rules, proxyDialer *net.Dialer) (*netstackBridge, error) {
	if mtu <= 0 {
		mtu = 1500
	}

	s := stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol, ipv6.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocolCUBIC, udp.NewProtocol, icmp.NewProtocol4, icmp.NewProtocol6},
	})

	ep := channel.New(1024, uint32(mtu), "")
	if err := s.CreateNIC(1, ep); err != nil {
		return nil, fmt.Errorf("create nic: %v", err)
	}
	if err := s.SetPromiscuousMode(1, true); err != nil {
		return nil, fmt.Errorf("set promiscuous mode: %v", err)
	}
	if err := s.SetSpoofing(1, true); err != nil {
		return nil, fmt.Errorf("set spoofing: %v", err)
	}

	// NIC address 10.0.0.2/24 — see comment on newNetstackBridge.
	addr := tcpip.AddrFrom4([4]byte{10, 0, 0, 2})
	if err := s.AddProtocolAddress(1, tcpip.ProtocolAddress{
		Protocol:          ipv4.ProtocolNumber,
		AddressWithPrefix: tcpip.AddressWithPrefix{Address: addr, PrefixLen: 24},
	}, stack.AddressProperties{}); err != nil {
		return nil, fmt.Errorf("add address: %v", err)
	}
	s.SetForwardingDefaultAndAllNICs(ipv4.ProtocolNumber, true)
	s.SetForwardingDefaultAndAllNICs(ipv6.ProtocolNumber, true)
	s.SetRouteTable([]tcpip.Route{
		{Destination: header.IPv4EmptySubnet, NIC: 1},
		{Destination: header.IPv6EmptySubnet, NIC: 1},
	})

	b := &netstackBridge{
		f:           f,
		localPort:   localPort,
		resolver:    resolver,
		rules:       ruleSet,
		cfg:         cfg,
		stk:         s,
		ep:          ep,
		proxyDialer: proxyDialer,
		done:        make(chan struct{}),
	}

	b.installForwarders()
	return b, nil
}

// installForwarders wires TCP (→ local proxy, via CONNECT synthesis) and
// UDP :53 (→ resolver backend).
func (b *netstackBridge) installForwarders() {
	// TCP: every flow is relayed to 127.0.0.1:localPort behind a synthesized
	// CONNECT naming the flow's original destination. The proxy then runs the
	// real MITM/SNI work; we just pipe bytes after the CONNECT handshake.
	tcpFwd := tcp.NewForwarder(b.stk, 0, 10000, func(r *tcp.ForwarderRequest) {
		defer func() {
			if rec := recover(); rec != nil {
				LogError("netstack: tcp forwarder panic: %v", rec)
			}
		}()
		// Capture the original destination NOW: r.ID() (and the Address slice
		// inside it) is only guaranteed valid inside this callback, and the
		// relay runs on its own goroutine. String() copies the bytes out.
		// LocalAddress/LocalPort are the flow's destination (what the app
		// dialed); RemoteAddress/RemotePort are the app socket's TUN-side
		// address:port — the source tuple needed for uid attribution.
		id := r.ID()
		flow := flowTuple{
			srcIP:   tcpipAddrToIP(id.RemoteAddress),
			dstIP:   tcpipAddrToIP(id.LocalAddress),
			srcPort: id.RemotePort,
			dstPort: id.LocalPort,
		}

		var wq waiter.Queue
		tep, err := r.CreateEndpoint(&wq)
		if err != nil {
			r.Complete(true)
			return
		}
		tep.SocketOptions().SetKeepAlive(true)
		tep.SocketOptions().SetDelayOption(false) // disable Nagle (TCP_NODELAY)
		r.Complete(false)
		conn := gonet.NewTCPConn(&wq, tep)
		go b.relayToProxy(conn, flow)
	})
	b.stk.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpFwd.HandlePacket)

	// UDP :53: DNS hijack. Other UDP is dropped (no relay target).
	udpFwd := udp.NewForwarder(b.stk, func(r *udp.ForwarderRequest) bool {
		defer func() {
			if rec := recover(); rec != nil {
				LogError("netstack: udp forwarder panic: %v", rec)
			}
		}()
		if r.ID().LocalPort != 53 {
			return false
		}
		var wq waiter.Queue
		uep, err := r.CreateEndpoint(&wq)
		if err != nil {
			return false
		}
		go b.handleDNS(gonet.NewUDPConn(&wq, uep))
		return true
	})
	b.stk.SetTransportProtocolHandler(udp.ProtocolNumber, udpFwd.HandlePacket)
}

type flowTuple struct {
	srcIP   net.IP
	dstIP   net.IP
	srcPort uint16
	dstPort uint16
}

func tcpipAddrToIP(addr tcpip.Address) net.IP {
	switch addr.Len() {
	case 4:
		a := addr.As4()
		return net.IP(a[:])
	case 16:
		a := addr.As16()
		return net.IP(a[:])
	default:
		return nil
	}
}

var (
	relayBufPool = sync.Pool{
		New: func() any {
			buf := make([]byte, 64*1024)
			return &buf
		},
	}
	dnsBufPool = sync.Pool{
		New: func() any {
			buf := make([]byte, 2048)
			return &buf
		},
	}
	connectReaderPool = sync.Pool{
		New: func() any {
			return bufio.NewReaderSize(nil, 1024)
		},
	}
)

// relayToProxy pipes a TUN-terminated TCP flow to the local proxy listener and
// back. The flow is opened with a synthesized CONNECT for dst ("ip:port" — the
// original destination the captured app dialed), so the proxy's handleConnect
// takes over: SNI-rewrite/MITM for :443, direct tunnel otherwise, upstream
// dials through its protect()-wrapped outbound dialer.
//
// src is the flow's TUN-side source ("ip:port"). Before the CONNECT goes out,
// the kernel is asked who owns that socket (inet_diag, see sockdiag): when
// resolvable, the uid rides along in an X-Snirect-Uid header so the proxy can
// attribute a client-side certificate rejection (pinning) back to the app.
func (b *netstackBridge) relayToProxy(client net.Conn, flow flowTuple) {
	defer client.Close()
	proxyAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(b.localPort))
	upstream, err := b.proxyDialer.Dial("tcp", proxyAddr)
	if err != nil {
		LogWarn("netstack: dial local proxy failed: %v", err)
		return
	}
	defer upstream.Close()

	dst := net.JoinHostPort(flow.dstIP.String(), strconv.Itoa(int(flow.dstPort)))
	src := net.JoinHostPort(flow.srcIP.String(), strconv.Itoa(int(flow.srcPort)))

	// Synthesize the proxy handshake (audit B1). Without this the proxy's
	// http.Server would parse the app's TLS ClientHello as a request line and
	// reject the flow; with it, the flow enters handleConnect like any
	// desktop CONNECT. X-Snirect-Uid carries flow attribution (netlink, or
	// the host getConnectionOwnerUid fallback for :443 — MITM is the only
	// consumer of the uid); X-Snirect-Src always carries the flow's source
	// tuple so the host can retry attribution at certificate-rejection time,
	// when the socket was certainly established.
	var reqBuf [256]byte
	req := reqBuf[:0]
	req = append(req, "CONNECT "...)
	req = append(req, dst...)
	req = append(req, " HTTP/1.1\r\nHost: "...)
	req = append(req, dst...)
	req = append(req, "\r\nX-Snirect-Src: "...)
	req = append(req, src...)
	req = append(req, "\r\n"...)
	if flow.dstPort == 443 {
		if uid := uidOfFlow(flow); uid > 0 {
			req = append(req, "X-Snirect-Uid: "...)
			req = strconv.AppendInt(req, int64(uid), 10)
			req = append(req, "\r\n"...)
		}
	}
	req = append(req, "\r\n"...)
	if _, err := upstream.Write(req); err != nil {
		LogWarn("netstack: send CONNECT %s failed: %v", dst, err)
		return
	}

	// Read the proxy's response through a bufio.Reader so any bytes it
	// buffered past the response headers (e.g. the first server flight of a
	// TLS handshake) are preserved for the relay below.
	br := connectReaderPool.Get().(*bufio.Reader)
	br.Reset(upstream)
	statusLine, err := br.ReadString('\n')
	if err != nil {
		br.Reset(nil)
		connectReaderPool.Put(br)
		LogWarn("netstack: read CONNECT %s response failed: %v", dst, err)
		return
	}
	if !strings.HasPrefix(statusLine, "HTTP/1.1 200") && !strings.HasPrefix(statusLine, "HTTP/1.0 200") {
		br.Reset(nil)
		connectReaderPool.Put(br)
		LogWarn("netstack: proxy refused CONNECT %s: %s", dst, strings.TrimSpace(statusLine))
		return
	}
	// Drain the remaining response headers up to the empty line.
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			br.Reset(nil)
			connectReaderPool.Put(br)
			LogWarn("netstack: truncated CONNECT %s response headers: %v", dst, err)
			return
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}

	LogDebug("netstack: CONNECT %s established", dst)
	if br.Buffered() == 0 {
		br.Reset(nil)
		connectReaderPool.Put(br)
		br = nil
	}
	pipe(client, br, upstream, dst)
}

func drainBuffered(dst io.Writer, br *bufio.Reader) (int64, error) {
	if br == nil {
		return 0, nil
	}
	buffered := br.Buffered()
	if buffered <= 0 {
		return 0, nil
	}
	peeked, err := br.Peek(buffered)
	if err != nil && len(peeked) == 0 {
		return 0, err
	}
	if len(peeked) == 0 {
		return 0, nil
	}
	n, werr := dst.Write(peeked)
	if n > 0 {
		_, _ = br.Discard(n)
	}
	return int64(n), werr
}

// pipe is a bidirectional byte copy between the TUN-side conn and the proxy
// upstream. If br has over-read bytes buffered past the CONNECT response, they
// are drained once before copying directly from upstream using pooled 64 KiB
// buffers (avoiding bufio.Reader.WriteTo's 4 KiB chunks). Returns when either
// direction finishes; the deferred Closes in relayToProxy then tear both ends.
// label names the flow in per-direction byte-count logs (debug level).
func pipe(client net.Conn, br *bufio.Reader, upstream net.Conn, label string) {
	done := make(chan struct{}, 2)
	go func() {
		bufPtr := relayBufPool.Get().(*[]byte)
		defer relayBufPool.Put(bufPtr)

		var total int64
		drained, err := drainBuffered(client, br)
		total += drained
		if err == nil {
			n, _ := io.CopyBuffer(client, upstream, *bufPtr)
			total += n
		}
		LogDebug("netstack: %s: proxy->client direction ended after %d bytes", label, total)
		if cw, ok := client.(interface{ CloseWrite() error }); ok {
			_ = cw.CloseWrite()
		}
		done <- struct{}{}
	}()
	go func() {
		bufPtr := relayBufPool.Get().(*[]byte)
		defer relayBufPool.Put(bufPtr)

		n, _ := io.CopyBuffer(upstream, client, *bufPtr)
		LogDebug("netstack: %s: client->proxy direction ended after %d bytes", label, n)
		if cw, ok := upstream.(interface{ CloseWrite() error }); ok {
			_ = cw.CloseWrite()
		}
		done <- struct{}{}
	}()
	<-done
}

// handleDNS answers a captured :53 UDP query using the shared resolver's
// backend (trusted upstream over the protect dialer). Ported from the deleted
// core/tun/dns.go, minus the AAAA-block (now driven by cfg.IPv6 at resolve
// time). Every failure path answers SERVFAIL — a silent drop makes clients
// hang on a black hole instead of retrying/falling back (audit B2).
func (b *netstackBridge) handleDNS(conn net.Conn) {
	defer func() {
		if rec := recover(); rec != nil {
			LogError("netstack: dns handler panic: %v", rec)
		}
		_ = conn.Close()
	}()

	servfail := func(q *dns.Msg) {
		reply := new(dns.Msg)
		reply.SetRcode(q, dns.RcodeServerFailure)
		writeDNS(conn, reply)
	}

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	bufPtr := dnsBufPool.Get().(*[]byte)
	n, err := conn.Read(*bufPtr)
	if err != nil {
		dnsBufPool.Put(bufPtr)
		return
	}
	msg := new(dns.Msg)
	unpackErr := msg.Unpack((*bufPtr)[:n])
	dnsBufPool.Put(bufPtr)
	if unpackErr != nil {
		servfail(msg)
		return
	}

	// AAAA blocking (restored from the deleted core/tun/dns.go): with the
	// engine v4-only, apps must not learn v6 addresses. The TUN carries a
	// v6 route whenever the service doesn't block them, and Android prefers
	// v6 destinations — every captured v6 flow then dials a GFW-blackholed
	// upstream through the proxy and dies (seen live: a carrier switchover
	// to an AAAA-answering DNS made google/hf unreachable while v4 worked).
	// Empty NOERROR = "name exists, no AAAA": the standard signal that makes
	// clients fall back to the A record.
	if len(msg.Question) > 0 && msg.Question[0].Qtype == dns.TypeAAAA &&
		(b.cfg == nil || !b.cfg.IPv6) {
		reply := new(dns.Msg)
		reply.SetReply(msg)
		writeDNS(conn, reply)
		return
	}

	// Host override: a rules.GetHost hit short-circuits to the rule's IP.
	if b.rules != nil && len(msg.Question) > 0 {
		qName := strings.TrimSuffix(msg.Question[0].Name, ".")
		if ipStr, ok := b.rules.GetHost(qName); ok {
			if ip := net.ParseIP(ipStr); ip != nil && msg.Question[0].Qtype == dns.TypeA {
				reply := new(dns.Msg)
				reply.SetReply(msg)
				if ip4 := ip.To4(); ip4 != nil {
					reply.Answer = []dns.RR{&dns.A{
						Hdr: dns.RR_Header{
							Name:   msg.Question[0].Name,
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    60,
						},
						A: ip4,
					}}
				}
				writeDNS(conn, reply)
				return
			}
		}
	}

	// Forward to the trusted resolver backend (protect-wrapped upstream).
	// One retry absorbs transient upstream blips — the fan-out races every
	// upstream and a single Exchange can still lose all of them for a moment
	// (observed on-device: one cold query SERVFAIL'd while the next attempt,
	// 100ms later, resolved in 6ms).
	for attempt := 0; ; attempt++ {
		if b.resolver == nil {
			servfail(msg)
			return
		}
		backend := b.resolver.Backend()
		if backend == nil {
			servfail(msg)
			return
		}
		reply, _, err := backend.Exchange(msg)
		if err == nil {
			writeDNS(conn, reply)
			return
		}
		if attempt == 0 {
			LogDebug("netstack: dns exchange failed (%v), retrying once", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		LogWarn("netstack: dns exchange failed twice for %s: %v", qnameOf(msg), err)
		servfail(msg)
		return
	}
}

// qnameOf extracts the first question name for logging; safe on any msg.
func qnameOf(msg *dns.Msg) string {
	if len(msg.Question) > 0 {
		return msg.Question[0].Name
	}
	return "?"
}

func writeDNS(conn net.Conn, msg *dns.Msg) {
	bufPtr := dnsBufPool.Get().(*[]byte)
	defer dnsBufPool.Put(bufPtr)
	data, err := msg.PackBuffer(*bufPtr)
	if err != nil {
		return
	}
	_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	_, _ = conn.Write(data)
}

// Run pumps packets between the TUN file and the gVisor stack until Close is
// called. It returns the fatal reason when the bridge died on its own (TUN
// read/write error — see fatal()), and nil when it was closed externally
// (StopEngine). The engine goroutine maps a non-nil return onto OnEngineError.
func (b *netstackBridge) Run() error {
	go b.readLoop()
	go b.writeLoop()
	<-b.done
	b.mu.Lock()
	reason := b.fatalReason
	b.mu.Unlock()
	if reason != "" {
		return fmt.Errorf("%s", reason)
	}
	return nil
}

// fatal records why the bridge died and closes it. Errors reaching this path
// mean the engine cannot continue (the TUN is gone or rejects our packets), so
// the reason must reach the UI — the old code logged at DEBUG and returned nil
// from Run, i.e. an engine that died with the notification still ACTIVE.
func (b *netstackBridge) fatal(format string, args ...interface{}) {
	reason := fmt.Sprintf(format, args...)
	LogError("netstack: fatal: %s", reason)
	b.mu.Lock()
	if b.fatalReason == "" && !b.closed {
		b.fatalReason = reason
	}
	b.mu.Unlock()
	_ = b.Close() // idempotent; unblocks Run
}

// readLoop pulls raw IP packets off the TUN file and injects them into gVisor.
func (b *netstackBridge) readLoop() {
	defer b.Close()
	buf := make([]byte, 65536+14)
	for {
		n, err := b.f.Read(buf)
		if err != nil {
			if b.isClosed() {
				// Expected teardown: StopEngine closed us (and/or the host
				// closed the fd) — not an engine death.
				LogDebug("netstack: tun read ended: %v", err)
				return
			}
			b.fatal("tun read failed: %v", err)
			return
		}
		if n == 0 {
			continue
		}
		ver := header.IPVersion(buf)
		var proto tcpip.NetworkProtocolNumber
		switch ver {
		case 4:
			proto = header.IPv4ProtocolNumber
		case 6:
			proto = header.IPv6ProtocolNumber
		default:
			continue
		}
		v := buffer.NewViewWithData(buf[:n])
		pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{Payload: buffer.MakeWithView(v)})
		b.ep.InjectInbound(proto, pkt)
		pkt.DecRef()
	}
}

func packetBytes(pkt *stack.PacketBuffer, scratch []byte) ([]byte, []byte) {
	slices := pkt.AsSlices()
	if len(slices) == 0 {
		return nil, scratch
	}
	if len(slices) == 1 {
		return slices[0], scratch
	}
	n := 0
	for _, s := range slices {
		if n+len(s) > len(scratch) {
			grown := make([]byte, n+len(s))
			copy(grown, scratch[:n])
			scratch = grown
		}
		n += copy(scratch[n:], s)
	}
	return scratch[:n], scratch
}

// writeLoop emits gVisor's outbound packets back to the TUN file.
func (b *netstackBridge) writeLoop() {
	defer b.Close()
	ctx := context.Background()
	scratch := make([]byte, 65536+14)
	for {
		pkt := b.ep.ReadContext(ctx)
		if pkt == nil {
			if b.isClosed() {
				return
			}
			continue
		}
		var data []byte
		data, scratch = packetBytes(pkt, scratch)
		var writeErr error
		if len(data) > 0 {
			_, writeErr = b.f.Write(data)
		}
		// DecRef MUST happen after b.f.Write completes because single-slice
		// packets alias pkt's backing memory directly.
		pkt.DecRef()
		if writeErr != nil {
			if b.isClosed() {
				LogDebug("netstack: tun write ended: %v", writeErr)
				return
			}
			// E.g. an oversized packet against a smaller VpnService MTU
			// (audit B3): every subsequent write would fail too.
			b.fatal("tun write failed: %v", writeErr)
			return
		}
	}
}

// Close stops the pumps and the gVisor stack. Idempotent. Whether this was an
// external stop or our own fatal path is distinguishable via fatalReason, which
// only fatal() sets — Close itself never marks the death.
func (b *netstackBridge) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	close(b.done)
	stk := b.stk
	ep := b.ep
	b.mu.Unlock()

	stk.Close()
	ep.Close()
	// Close our Dup'd TUN fd (see newTunFile). Unblocks a blocked Read in
	// readLoop. Idempotent at this layer: a second Close returns above.
	//
	// b.f is deliberately NOT nilled here. It is immutable after
	// construction, so the pumps' unlocked b.f.Read/b.f.Write calls can never
	// see nil — nilling it raced a writeLoop holding a packet mid-flight
	// (b.f nilled between its ReadContext return and its Write) and crashed
	// the whole process with a nil-interface SIGSEGV at the Write. A closed
	// *os.File only ever returns ErrClosed from these calls, never panics,
	// and both pumps check isClosed() on that error path and exit.
	_ = b.f.Close()
	return nil
}

func (b *netstackBridge) isClosed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closed
}
