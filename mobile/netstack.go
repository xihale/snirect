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

	coreconfig "github.com/xihale/snirect/config"
	coredns "github.com/xihale/snirect/dns"
	"github.com/xihale/snirect/rules"

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
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol, icmp.NewProtocol4, icmp.NewProtocol6},
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
		id := r.ID()
		dst := net.JoinHostPort(id.LocalAddress.String(), strconv.Itoa(int(id.LocalPort)))

		var wq waiter.Queue
		tep, err := r.CreateEndpoint(&wq)
		if err != nil {
			r.Complete(true)
			return
		}
		tep.SocketOptions().SetKeepAlive(true)
		r.Complete(false)
		conn := gonet.NewTCPConn(&wq, tep)
		go b.relayToProxy(conn, dst)
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

// relayToProxy pipes a TUN-terminated TCP flow to the local proxy listener and
// back. The flow is opened with a synthesized CONNECT for dst ("ip:port" — the
// original destination the captured app dialed), so the proxy's handleConnect
// takes over: SNI-rewrite/MITM for :443, direct tunnel otherwise, upstream
// dials through its protect()-wrapped outbound dialer.
func (b *netstackBridge) relayToProxy(client net.Conn, dst string) {
	defer client.Close()
	proxyAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(b.localPort))
	upstream, err := b.proxyDialer.Dial("tcp", proxyAddr)
	if err != nil {
		LogWarn("netstack: dial local proxy failed: %v", err)
		return
	}
	defer upstream.Close()

	// Synthesize the proxy handshake (audit B1). Without this the proxy's
	// http.Server would parse the app's TLS ClientHello as a request line and
	// reject the flow; with it, the flow enters handleConnect like any
	// desktop CONNECT.
	req := "CONNECT " + dst + " HTTP/1.1\r\nHost: " + dst + "\r\n\r\n"
	if _, err := upstream.Write([]byte(req)); err != nil {
		LogWarn("netstack: send CONNECT %s failed: %v", dst, err)
		return
	}

	// Read the proxy's response through a bufio.Reader so any bytes it
	// buffered past the response headers (e.g. the first server flight of a
	// TLS handshake) are preserved for the relay below.
	br := bufio.NewReader(upstream)
	statusLine, err := br.ReadString('\n')
	if err != nil {
		LogWarn("netstack: read CONNECT %s response failed: %v", dst, err)
		return
	}
	if !strings.HasPrefix(statusLine, "HTTP/1.1 200") && !strings.HasPrefix(statusLine, "HTTP/1.0 200") {
		LogWarn("netstack: proxy refused CONNECT %s: %s", dst, strings.TrimSpace(statusLine))
		return
	}
	// Drain the remaining response headers up to the empty line.
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			LogWarn("netstack: truncated CONNECT %s response headers: %v", dst, err)
			return
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}

	LogDebug("netstack: CONNECT %s established", dst)
	pipe(client, br, upstream, dst)
}

// pipe is a bidirectional byte copy between the TUN-side conn and the proxy
// upstream. It reads the proxy→client direction from proxySrc (the bufio.Reader
// wrapping upstream, preserving its over-read past the CONNECT response) and
// writes the client→proxy direction straight to proxyDst. Returns when either
// direction finishes; the deferred Closes in relayToProxy then tear both ends.
// label names the flow in per-direction byte-count logs (debug level).
func pipe(client net.Conn, proxySrc io.Reader, proxyDst io.Writer, label string) {
	done := make(chan struct{}, 2)
	cp := func(dir string, dst io.Writer, src io.Reader) {
		n, _ := io.Copy(dst, src)
		LogDebug("netstack: %s: %s direction ended after %d bytes", label, dir, n)
		if cw, ok := dst.(interface{ CloseWrite() error }); ok {
			_ = cw.CloseWrite()
		}
		done <- struct{}{}
	}
	go cp("proxy->client", client, proxySrc)
	go cp("client->proxy", proxyDst, client)
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
	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}
	msg := new(dns.Msg)
	if err := msg.Unpack(buf[:n]); err != nil {
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
		qName := dns.Fqdn(msg.Question[0].Name)
		if ip, ok := b.rules.GetHost(qName); ok && net.ParseIP(ip) != nil {
			if msg.Question[0].Qtype == dns.TypeA {
				reply := new(dns.Msg)
				reply.SetReply(msg)
				if rr, err := dns.NewRR(fmt.Sprintf("%s 60 IN A %s", msg.Question[0].Name, ip)); err == nil {
					reply.Answer = append(reply.Answer, rr)
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
	data, err := msg.Pack()
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

// writeLoop emits gVisor's outbound packets back to the TUN file.
func (b *netstackBridge) writeLoop() {
	defer b.Close()
	ctx := context.Background()
	for {
		pkt := b.ep.ReadContext(ctx)
		if pkt == nil {
			if b.isClosed() {
				return
			}
			continue
		}
		data := pkt.ToView().AsSlice()
		pkt.DecRef()
		if len(data) > 0 {
			if _, err := b.f.Write(data); err != nil {
				if b.isClosed() {
					LogDebug("netstack: tun write ended: %v", err)
					return
				}
				// E.g. an oversized packet against a smaller VpnService MTU
				// (audit B3): every subsequent write would fail too.
				b.fatal("tun write failed: %v", err)
				return
			}
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
