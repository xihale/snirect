// Package proxy implements the core HTTP/HTTPS proxy server with TLS interception
// and SNI modification capabilities. It handles CONNECT tunneling, direct forwarding,
// and certificate management for MITM decryption.
package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/xihale/snirect/config"
	"github.com/xihale/snirect/logger"
	"github.com/xihale/snirect/rules"
)

// CertificateManager manages root CA and signs leaf certificates.
type CertificateManager interface {
	GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error)
	GetRootCACertPEM() []byte
	Close() error
}

// Resolver resolves hostnames to IP addresses with caching.
type Resolver interface {
	Resolve(ctx context.Context, host string, clientIP net.IP) (string, error)
	Invalidate(host string)
	Close() error
}

// ProxyServer handles HTTP and HTTPS proxying.
type ProxyServer struct {
	Config    *config.Config
	Rules     *rules.Rules
	CA        CertificateManager
	Resolver  Resolver
	semaphore chan struct{} //Limits concurrent connections
	server    *http.Server
	listener  net.Listener

	// outboundDialer, when set, is used for all upstream TCP dials. When nil,
	// a fresh &net.Dialer{Timeout: cfg.Timeout.Dial} is used (the historical
	// desktop behavior). Mobile (Android VpnService) injects a protect()-wrapped
	// dialer so the proxy's own upstream traffic escapes the TUN instead of
	// looping back into the capture route.
	outboundDialer *net.Dialer
}

// NewProxyServer creates a new ProxyServer instance.
func NewProxyServer(cfg *config.Config, rules *rules.Rules, ca CertificateManager, resolver Resolver) *ProxyServer {
	var sem chan struct{}
	if cfg.Limit.MaxConns > 0 {
		sem = make(chan struct{}, cfg.Limit.MaxConns)
	}
	return &ProxyServer{
		Config:    cfg,
		Rules:     rules,
		CA:        ca,
		Resolver:  resolver,
		semaphore: sem,
	}
}

// SetOutboundDialer injects the dialer used for upstream TCP connections. It is
// optional: desktop callers leave it nil and get a default net.Dialer; mobile
// callers pass a protect()-wrapped dialer so upstreams bypass the VPN TUN.
// Must be called before Listen/Serve.
func (s *ProxyServer) SetOutboundDialer(d *net.Dialer) { s.outboundDialer = d }

// dialOutbound dials a TCP address using the injected dialer if present,
// otherwise a fresh net.Dialer with the configured dial timeout. The injected
// dialer's own Control hook (e.g. VpnService.protect) is preserved.
func (s *ProxyServer) dialOutbound(ctx context.Context, addr string, timeout time.Duration) (net.Conn, error) {
	if s.outboundDialer != nil {
		// Honor the per-call timeout without clobbering the injected dialer's
		// Control/KeepAlive config: copy and override Timeout only.
		d := *s.outboundDialer
		if timeout > 0 {
			d.Timeout = timeout
		}
		return d.DialContext(ctx, "tcp", addr)
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", addr)
}

// Listen binds the configured address/port, records the actual port (which
// matters when Config.Server.Port == 0 asks for an auto-picked one), and
// prepares the http.Server. It returns once the listener is ready, so callers
// know the real port before serving begins. It does not block.
func (s *ProxyServer) Listen() error {
	addr := fmt.Sprintf("%s:%d", s.Config.Server.Address, s.Config.Server.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = ln
	if s.Config.Server.Port == 0 {
		s.Config.Server.Port = ln.Addr().(*net.TCPAddr).Port
	}
	s.server = &http.Server{Handler: s}
	logger.System().Info("proxy server listening", "addr", ln.Addr().String())
	return nil
}

// Serve accepts connections until Shutdown or an error. It blocks. Must be
// called after Listen.
func (s *ProxyServer) Serve() error {
	err := s.server.Serve(s.listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown gracefully shuts down the proxy server, waiting for active connections to complete.
func (s *ProxyServer) Shutdown(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// ServeHTTP handles HTTP requests by routing CONNECT to the proxy handler
// and other requests to the HTTP handler for PAC/cert/redirect.
func (s *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		s.handleConnect(w, r)
	} else {
		s.handleHTTP(w, r)
	}
}

// handleHTTP handles standard HTTP requests (PAC, Cert download, or HTTP->HTTPS redirect).
func (s *ProxyServer) handleHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasPrefix(r.URL.Path, "/pac/"):
		s.handlePAC(w, r)
	case strings.HasPrefix(r.URL.Path, "/CERT/root."):
		s.handleCertDownload(w, r)
	default:
		// Redirect HTTP to HTTPS
		targetURL := "https://" + strings.TrimPrefix(r.URL.String(), "http://")
		http.Redirect(w, r, targetURL, http.StatusMovedPermanently)
	}
}

func (s *ProxyServer) handlePAC(w http.ResponseWriter, r *http.Request) {
	setLocalEndpointHeaders(w)
	w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")

	appDir, _ := config.GetAppDataDir()
	pacPath := filepath.Join(appDir, "pac")
	port := s.Config.Server.Port

	if content, err := os.ReadFile(pacPath); err == nil {
		sContent := strings.ReplaceAll(string(content), "{{port}}", fmt.Sprintf("%d", port))
		sContent = strings.ReplaceAll(sContent, "{{host}}", s.Config.Server.PACHost)
		_, _ = w.Write([]byte(sContent))
		return
	}

	pacContent := fmt.Sprintf(`function FindProxyForURL(url, host) { return "PROXY %s:%d"; }`, s.Config.Server.PACHost, port)
	_, _ = w.Write([]byte(pacContent))
}

func (s *ProxyServer) handleCertDownload(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRemote(r.RemoteAddr) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	setLocalEndpointHeaders(w)
	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	_, _ = w.Write(s.CA.GetRootCACertPEM())
}

func setLocalEndpointHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

func isLoopbackRemote(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// handleConnect handles the HTTP CONNECT method for HTTPS tunneling.
func (s *ProxyServer) handleConnect(w http.ResponseWriter, r *http.Request) {
	if s.semaphore != nil {
		s.semaphore <- struct{}{}
		defer func() { <-s.semaphore }()
	}

	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
		port = "443"
	}

	// 1. Hijack connection
	clientConn, err := s.hijackConnection(w)
	if err != nil {
		logger.Client().Debug("hijack failed", "host", host, "error", err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	// From this point on, we are responsible for closing clientConn.
	// We delegate closure to the tunnel function or close on error paths.

	// 2. Respond 200 OK to client
	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		clientConn.Close()
		return
	}

	// 3. Determine if we should intercept (MITM). Transparent flows carry a
	//    bare address (tun2socks CONNECT synthesis), so the hostname rules are
	//    keyed on the ClientHello SNI: peek it first and route by it. The
	//    replay conn keeps the stream intact whichever branch runs.
	if net.ParseIP(host) != nil && port == "443" {
		replay, sni, perr := peekClientHelloSNI(clientConn)
		clientConn = replay
		if perr != nil || !s.shouldIntercept(sni, port) {
			// Not a TLS ClientHello, or no rule wants it — splice through
			// untouched (desktop direct-tunnel semantics for rule-less hosts).
			s.directTunnel(r.Context(), replay, host, port)
			return
		}
	} else if !s.shouldIntercept(host, port) {
		s.directTunnel(r.Context(), clientConn, host, port)
		return
	}

	// 4. MITM with ALPN coordination. The client- and upstream-side TLS
	// handshakes run back-to-back inside GetConfigForClient so that the ALPN
	// protocol we advertise to the client is whatever the upstream actually
	// negotiated. A blind byte tunnel is only transparent when both ends agree
	// on the application protocol (h2 on both sides, or http/1.1 on both);
	// otherwise the client expects an HTTP/2 SETTINGS frame and receives
	// HTTP/1.1 text (curl error 16).
	var remoteConn *tls.Conn

	clientTLS := &tls.Config{
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			clientSNI := hello.ServerName
			if clientSNI == "" {
				clientSNI = host
			}
			targetSNI := s.determineSNI(host, clientSNI)

			// Offer the upstream exactly the protocols the client offered, so
			// the upstream's choice is necessarily something the client can
			// also speak. If the client offered nothing, pin http/1.1.
			// GitHub archive intern-follow parses HTTP/1.1, so pin it there.
			offer := upstreamOffer(hello.SupportedProtos)
			if isGitHubHTTPHost(clientSNI) || isGitHubHTTPHost(host) {
				offer = []string{"http/1.1"}
			}
			rc, err := s.connectToRemote(r.Context(), host, port, r.RemoteAddr, targetSNI, offer)
			if err != nil {
				return nil, err // connectToRemote already logged (DNS/dial/handshake)
			}

			if !s.verifyServerCert(rc, clientSNI, targetSNI) {
				state := rc.ConnectionState()
				var certInfo string
				if len(state.PeerCertificates) > 0 {
					cert := state.PeerCertificates[0]
					if len(cert.DNSNames) > 0 {
						certInfo = fmt.Sprintf("server cert domains: %v", cert.DNSNames)
					} else {
						certInfo = fmt.Sprintf("server cert subject: %s", cert.Subject.CommonName)
					}
				} else {
					certInfo = "no certificates provided by server"
				}
				logger.Upstream().Warn("upstream certificate rejected", "host", host, "target_sni", targetSNI, "cert_info", certInfo)
				rc.Close()
				return nil, errors.New("upstream certificate rejected")
			}

			remoteConn = rc
			chosen := pickALPN(remoteConn.ConnectionState().NegotiatedProtocol)

			// Per-connection config: selects the leaf cert (with SNI fallback
			// to the CONNECT host) and pins the advertised ALPN to the
			// upstream's negotiated protocol, guaranteeing both sides match.
			return &tls.Config{
				GetCertificate: func(h *tls.ClientHelloInfo) (*tls.Certificate, error) {
					if h.ServerName == "" {
						h.ServerName = host
					}
					return s.CA.GetCertificate(h)
				},
				NextProtos: []string{chosen},
			}, nil
		},
	}

	// 5. Drive the client-side handshake. The callback above performs the
	// upstream connect + cert verification; on its failure the handshake
	// aborts and we clean up any half-opened upstream connection.
	tlsClientConn := tls.Server(clientConn, clientTLS)
	if err := tlsClientConn.Handshake(); err != nil {
		// A pinned client rejects our leaf cert and hangs up (EOF), or the
		// handshake fails for some other reason (timeout, network noise).
		// This connection is mid-handshake and cannot be salvaged, so close it.
		logger.Client().Warn("client TLS handshake failed", "host", host, "error", err)
		if remoteConn != nil {
			remoteConn.Close()
		}
		clientConn.Close()
		return
	}

	// 6. GitHub archive/codeload: intern-follow the 302 so the client never
	// hits the broken dedicated-vhost 301→404. Other MITM hosts stay a pipe.
	logger.Upstream().Debug("tls tunnel established", "host", host, "alpn", remoteConn.ConnectionState().NegotiatedProtocol)
	clientSNI := tlsClientConn.ConnectionState().ServerName
	if clientSNI == "" {
		clientSNI = host
	}
	if isGitHubHTTPHost(clientSNI) || isGitHubHTTPHost(host) {
		s.serveGitHubH1(tlsClientConn, remoteConn, clientSNI, r.RemoteAddr, r.Context())
		return
	}
	s.tunnel(tlsClientConn, remoteConn)
}

func (s *ProxyServer) hijackConnection(w http.ResponseWriter) (net.Conn, error) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("hijacking not supported")
	}
	conn, _, err := hijacker.Hijack()
	return conn, err
}

func (s *ProxyServer) shouldIntercept(host, port string) bool {
	// Only intercept port 443
	if port != "443" {
		return false
	}

	// Check rules
	_, hasAlter := s.Rules.GetAlterHostname(host)
	policy, hasCert := s.Rules.GetCertVerify(host)

	// If no specific rule, use global setting
	if !hasCert {
		policy = s.Config.CheckHostnamePolicy()
	}

	// If global verification is enabled (policy.Enabled == true) AND no SNI modification is needed,
	// we can bypass MITM (Direct Tunnel).
	// Logic: We only MITM if we *need* to modify SNI or if we want to bypass cert verification (policy.Enabled == false).
	// If policy.Enabled is TRUE, we might still MITM if we need to modify SNI.
	return hasAlter || !policy.Enabled
}

// upstreamOffer derives the ALPN protocol list to offer the upstream from the
// client's ClientHello. The upstream is offered exactly the protocols the
// client supports, so whatever it negotiates is something the client can also
// speak. An empty client offer means ALPN is off; we still offer http/1.1 so
// the upstream handshake completes and we end up with a single concrete
// protocol to mirror back to the client.
func upstreamOffer(clientProtos []string) []string {
	if len(clientProtos) > 0 {
		return clientProtos
	}
	return []string{"http/1.1"}
}

// pickALPN normalizes the upstream-negotiated protocol into the single value
// we mirror to the client. Empty falls back to http/1.1.
func pickALPN(negotiated string) string {
	if negotiated == "h2" {
		return "h2"
	}
	return "http/1.1"
}

func (s *ProxyServer) determineSNI(host, clientHelloHost string) string {
	targetSNI, ok := s.Rules.GetAlterHostname(clientHelloHost)
	if !ok {
		return clientHelloHost
	}
	if targetSNI == "" {
		logger.Upstream().Debug("stripping SNI", "host", host)
	} else if targetSNI != clientHelloHost {
		logger.Upstream().Debug("replacing SNI", "host", host, "original_sni", clientHelloHost, "target_sni", targetSNI)
	}
	return targetSNI
}

func (s *ProxyServer) connectToRemote(ctx context.Context, host, port, clientAddr, targetSNI string, alpnOffer []string) (*tls.Conn, error) {
	// Resolve IP
	clientIP, _, _ := net.SplitHostPort(clientAddr)
	remoteIP, err := s.Resolver.Resolve(ctx, host, net.ParseIP(clientIP))
	if err != nil {
		logger.DNS().Warn("DNS resolution failed", "host", host, "error", err)
		return nil, fmt.Errorf("DNS resolution failed: %w", err)
	}
	remoteAddr := net.JoinHostPort(remoteIP, port)

	// Dial TCP. dialOutbound routes through the injected outbound dialer when
	// one is present (mobile), else a default net.Dialer (desktop).
	timeout := time.Duration(s.Config.Timeout.Dial) * time.Second
	netConn, err := s.dialOutbound(ctx, remoteAddr, timeout)
	if err != nil {
		s.Resolver.Invalidate(host)
		logger.Upstream().Warn("cannot reach upstream", "host", host, "addr", remoteAddr, "error", err)
		return nil, fmt.Errorf("dial failed to %s: %w", remoteAddr, err)
	}

	// Handshake TLS. ALPN offer is coordinated by the caller: the upstream is
	// offered only protocols the client supports, so its negotiated protocol
	// is something the client can also speak.
	remoteConn := tls.Client(netConn, &tls.Config{
		ServerName:         targetSNI,
		InsecureSkipVerify: true, // We verify manually
		NextProtos:         alpnOffer,
	})

	if err := remoteConn.Handshake(); err != nil {
		netConn.Close()
		logger.Upstream().Warn("upstream TLS handshake failed", "host", host, "error", err)
		return nil, fmt.Errorf("remote handshake failed: %w", err)
	}

	return remoteConn, nil
}

// verifyServerCert checks the upstream connection's certificate. hostname is
// the client-facing name the rules key on — the ClientHello SNI (which equals
// the CONNECT host for browser traffic), never the bare address of a
// transparent flow, which would miss every hostname rule.
func (s *ProxyServer) verifyServerCert(conn *tls.Conn, hostname, targetSNI string) bool {
	policy, ok := s.Rules.GetCertVerify(hostname)
	if !ok {
		policy = s.Config.CheckHostnamePolicy()
	}
	ignoreExpiry := s.Rules.GetIgnoreExpiry(hostname)
	return VerifyCert(conn, hostname, targetSNI, policy, s.Config.Security, ignoreExpiry)
}

func (s *ProxyServer) directTunnel(ctx context.Context, clientConn net.Conn, host, port string) {
	clientIP, _, _ := net.SplitHostPort(clientConn.RemoteAddr().String())
	remoteIP, err := s.Resolver.Resolve(ctx, host, net.ParseIP(clientIP))
	if err != nil {
		logger.DNS().Warn("DNS resolution failed", "host", host, "error", err)
		clientConn.Close()
		return
	}

	remoteAddr := net.JoinHostPort(remoteIP, port)
	timeout := time.Duration(s.Config.Timeout.Dial) * time.Second
	remoteConn, err := s.dialOutbound(ctx, remoteAddr, timeout)
	if err != nil {
		logger.Upstream().Warn("cannot reach upstream", "host", host, "addr", remoteAddr, "error", err)
		clientConn.Close()
		return
	}

	logger.Upstream().Debug("direct tunnel established", "host", host)
	s.tunnel(clientConn, remoteConn)
}

// tunnelIdle is how long a tunneled connection may sit with no bytes before
// we tear it down. Absolute caps used to kill large GitHub/release downloads
// that still had data flowing.
const tunnelIdle = 5 * time.Minute

// tunnel pipes data between c1 and c2. It closes both connections when done.
func (s *ProxyServer) tunnel(c1, c2 net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	bufSize := s.Config.Server.BufferSize
	if bufSize <= 0 {
		bufSize = 65536
	}
	if bufSize < 4096 {
		bufSize = 4096
	}
	if bufSize > 1048576 {
		bufSize = 1048576
	}

	pipe := func(dst, src net.Conn) {
		defer wg.Done()
		defer func() {
			c1.Close()
			c2.Close()
		}()
		buf := make([]byte, bufSize)
		for {
			_ = src.SetReadDeadline(time.Now().Add(tunnelIdle))
			n, err := src.Read(buf)
			if n > 0 {
				if _, werr := dst.Write(buf[:n]); werr != nil {
					if !errors.Is(werr, net.ErrClosed) {
						logger.Upstream().Debug("tunnel copy error", "error", werr)
					}
					return
				}
			}
			if err != nil {
				if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) && !isTimeout(err) {
					logger.Upstream().Debug("tunnel copy error", "error", err)
				}
				if cw, ok := dst.(interface{ CloseWrite() error }); ok {
					_ = cw.CloseWrite()
				}
				return
			}
		}
	}

	go pipe(c1, c2)
	go pipe(c2, c1)
	wg.Wait()
}

func isTimeout(err error) bool {
	var nerr net.Error
	return errors.As(err, &nerr) && nerr.Timeout()
}
