package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"snirect/internal/ca"
	"snirect/internal/config"
	"snirect/internal/dns"
	"snirect/internal/logger"
	"snirect/internal/tlsutil"
	"strings"
	"sync"
	"time"
)

// ProxyServer handles HTTP and HTTPS proxying.
type ProxyServer struct {
	Config    *config.Config
	Rules     *config.Rules
	CA        *ca.CertManager
	Resolver  *dns.Resolver
	semaphore chan struct{} // Limits concurrent connections
}

// NewProxyServer creates a new instance of ProxyServer.
func NewProxyServer(cfg *config.Config, rules *config.Rules, ca *ca.CertManager) *ProxyServer {
	var sem chan struct{}
	if cfg.Limit.MaxConns > 0 {
		sem = make(chan struct{}, cfg.Limit.MaxConns)
	}

	return &ProxyServer{
		Config:    cfg,
		Rules:     rules,
		CA:        ca,
		Resolver:  dns.NewResolver(cfg, rules),
		semaphore: sem,
	}
}

// Start runs the proxy server on the configured address and port.
func (s *ProxyServer) Start() error {
	addr := fmt.Sprintf("%s:%d", s.Config.Server.Address, s.Config.Server.Port)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	if s.Config.Server.Port == 0 {
		actualAddr := ln.Addr().(*net.TCPAddr)
		s.Config.Server.Port = actualAddr.Port
	}

	logger.Info("Serving on %s", ln.Addr().String())
	return http.Serve(ln, s)
}

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
	w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")

	appDir, _ := config.GetAppDataDir()
	pacPath := filepath.Join(appDir, "pac")

	if content, err := os.ReadFile(pacPath); err == nil {
		sContent := strings.ReplaceAll(string(content), "{{port}}", fmt.Sprintf("%d", s.Config.Server.Port))
		sContent = strings.ReplaceAll(sContent, "{{host}}", s.Config.Server.PACHost)
		w.Write([]byte(sContent))
		return
	}

	pacContent := fmt.Sprintf(`function FindProxyForURL(url, host) { return "PROXY %s:%d"; }`, s.Config.Server.PACHost, s.Config.Server.Port)
	w.Write([]byte(pacContent))
}

func (s *ProxyServer) handleCertDownload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	w.Write(s.CA.GetRootCACertPEM())
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
		logger.Debug("Hijack failed for %s: %v", r.RemoteAddr, err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	// From this point on, we are responsible for closing clientConn.
	// We delegate closure to the tunnel function or close on error.
	defer func() {
		// This defer is a safety net. The tunnel function typically closes connections.
		// We only want to close here if we return early before establishing the tunnel.
		// However, checking if it's closed is hard.
		// The standard pattern is to hand off responsibility.
		// For simplicity in this flow, we will ensure 'tunnel' closes them,
		// and we close explicitly if we error before tunnel.
	}()

	// 2. Respond 200 OK to client
	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		clientConn.Close()
		return
	}

	// 3. Determine if we should intercept (MITM)
	if !s.shouldIntercept(host, port) {
		s.directTunnel(r.Context(), clientConn, host, port)
		return
	}

	// 4. Perform TLS Handshake (Server-side) to get ClientHello
	tlsClientConn, clientHelloHost, err := s.handshakeClient(clientConn, host)
	if err != nil {
		logger.Warn("TLS Handshake with client failed: %v", err)
		clientConn.Close()
		return
	}

	// 5. Connect to Remote (Client-side)
	targetSNI := s.determineSNI(host, clientHelloHost)
	remoteConn, err := s.connectToRemote(r.Context(), host, port, r.RemoteAddr, targetSNI)
	if err != nil {
		logger.Warn("Failed to connect to remote %s: %v", host, err)
		tlsClientConn.Close()
		return
	}

	// 6. Verify Remote Certificate
	if !s.verifyServerCert(remoteConn, host) {
		logger.Warn("Certificate verification failed for %s", host)
		tlsClientConn.Close()
		remoteConn.Close()
		return
	}

	// 7. Tunnel Data
	logger.Info("Tunnel: %s <-> %s (SNI: %s)", r.RemoteAddr, host, targetSNI)
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
		policy, _ = config.ParseCertPolicy(s.Config.CheckHostname)
	}

	// If global verification is enabled (policy.Enabled == true) AND no SNI modification is needed,
	// we can bypass MITM (Direct Tunnel).
	// Logic: We only MITM if we *need* to modify SNI or if we want to bypass cert verification (policy.Enabled == false).
	// If policy.Enabled is TRUE, we might still MITM if we need to modify SNI.
	return hasAlter || !policy.Enabled
}

func (s *ProxyServer) handshakeClient(clientConn net.Conn, defaultHost string) (*tls.Conn, string, error) {
	tlsConfig := &tls.Config{
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			if hello.ServerName == "" {
				hello.ServerName = defaultHost
			}
			return s.CA.GetCertificate(hello)
		},
	}
	tlsConn := tls.Server(clientConn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		return nil, "", err
	}

	sni := tlsConn.ConnectionState().ServerName
	if sni == "" {
		sni = defaultHost
	}
	return tlsConn, sni, nil
}

func (s *ProxyServer) determineSNI(host, clientHelloHost string) string {
	targetSNI, ok := s.Rules.GetAlterHostname(clientHelloHost)
	if !ok {
		return clientHelloHost
	}
	if targetSNI == "" {
		logger.Debug("Stripping SNI for %s", host)
	} else if targetSNI != clientHelloHost {
		logger.Debug("Replacing SNI for %s with %s", host, targetSNI)
	}
	return targetSNI
}

func (s *ProxyServer) connectToRemote(ctx context.Context, host, port, clientAddr, targetSNI string) (*tls.Conn, error) {
	// Resolve IP
	clientIP, _, _ := net.SplitHostPort(clientAddr)
	remoteIP, err := s.Resolver.Resolve(ctx, host, net.ParseIP(clientIP))
	if err != nil {
		return nil, fmt.Errorf("DNS resolution failed: %w", err)
	}
	remoteAddr := net.JoinHostPort(remoteIP, port)

	// Dial TCP
	timeout := time.Duration(s.Config.Timeout.Dial) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	dialer := &net.Dialer{Timeout: timeout}
	netConn, err := dialer.DialContext(ctx, "tcp", remoteAddr)
	if err != nil {
		s.Resolver.Invalidate(host)
		return nil, fmt.Errorf("dial failed to %s: %w", remoteAddr, err)
	}

	// Handshake TLS
	remoteConn := tls.Client(netConn, &tls.Config{
		ServerName:         targetSNI,
		InsecureSkipVerify: true, // We verify manually
	})

	if err := remoteConn.Handshake(); err != nil {
		netConn.Close()
		return nil, fmt.Errorf("remote handshake failed: %w", err)
	}

	return remoteConn, nil
}

func (s *ProxyServer) verifyServerCert(conn *tls.Conn, host string) bool {
	policy, ok := s.Rules.GetCertVerify(host)
	if !ok {
		policy, _ = config.ParseCertPolicy(s.Config.CheckHostname)
	}

	// Fast path for skipping
	if !policy.Enabled {
		return true
	}

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return false
	}
	cert := state.PeerCertificates[0]

	// Check against specific domains if policy has Allowed list
	if len(policy.Allowed) > 0 {
		for _, dStr := range policy.Allowed {
			if tlsutil.MatchHostname(cert, dStr, policy) {
				return true
			}
		}
		logger.Debug("Cert domains %v did not match allowed list %v", cert.DNSNames, policy.Allowed)
		return false
	}

	// Standard check
	if !tlsutil.MatchHostname(cert, host, policy) {
		logger.Debug("Hostname %s does not match cert domains %v", host, cert.DNSNames)
		return false
	}

	return true
}

func (s *ProxyServer) directTunnel(ctx context.Context, clientConn net.Conn, host, port string) {
	clientIP, _, _ := net.SplitHostPort(clientConn.RemoteAddr().String())
	remoteIP, err := s.Resolver.Resolve(ctx, host, net.ParseIP(clientIP))
	if err != nil {
		logger.Warn("Direct Tunnel: DNS failed for %s: %v", host, err)
		clientConn.Close()
		return
	}

	remoteAddr := net.JoinHostPort(remoteIP, port)
	timeout := time.Duration(s.Config.Timeout.Dial) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	dialer := &net.Dialer{Timeout: timeout}
	remoteConn, err := dialer.DialContext(ctx, "tcp", remoteAddr)
	if err != nil {
		logger.Warn("Direct Tunnel: Connect failed %s: %v", remoteAddr, err)
		clientConn.Close()
		return
	}

	logger.Info("Direct Tunnel: %s <-> %s", clientConn.RemoteAddr(), remoteAddr)
	s.tunnel(clientConn, remoteConn)
}

// tunnel pipes data between c1 and c2. It closes both connections when done.
func (s *ProxyServer) tunnel(c1, c2 net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	pipe := func(dst, src net.Conn) {
		defer wg.Done()
		// We ignore errors here as they usually mean "connection closed"
		io.Copy(dst, src)
		// Close the write side of the destination if possible,
		// otherwise, we can only rely on the final close.
		// For standard net.Conn, Close() shuts down both sides.
		// To properly teardown, usually closing one side's read causes the other's copy to return.
		// We'll use a crude "close everything on finish" approach which is standard for simple proxies.
		// A better approach requires CloseWrite support.
		if cw, ok := dst.(interface{ CloseWrite() error }); ok {
			cw.CloseWrite()
		} else {
			// If we can't half-close, we might have to hard close,
			// but doing so might cut off the other direction's in-flight data.
			// However, for HTTPS proxies, strict TCP termination isn't always perfect.
			// Let's rely on the defer below to Close() both.
		}
	}

	go pipe(c1, c2)
	go pipe(c2, c1)

	wg.Wait()
	c1.Close()
	c2.Close()
}
