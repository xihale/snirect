package proxy

import (
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
	"time"
)

type ProxyServer struct {
	Config   *config.Config
	Rules    *config.Rules
	CA       *ca.CertManager
	Resolver *dns.Resolver
}

func NewProxyServer(cfg *config.Config, rules *config.Rules, ca *ca.CertManager) *ProxyServer {
	return &ProxyServer{
		Config:   cfg,
		Rules:    rules,
		CA:       ca,
		Resolver: dns.NewResolver(cfg, rules),
	}
}

func (s *ProxyServer) Start() error {
	addr := fmt.Sprintf("%s:%d", s.Config.Server.Address, s.Config.Server.Port)
	logger.Info("Serving on %s", addr)
	return http.ListenAndServe(addr, s)
}

func (s *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		s.handleConnect(w, r)
	} else {
		s.handleHTTP(w, r)
	}
}

func (s *ProxyServer) handleHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle PAC
	if strings.HasPrefix(r.URL.Path, "/pac/") {
		s.handlePAC(w, r)
		return
	}
	// Handle CA Download
	if strings.HasPrefix(r.URL.Path, "/CERT/root.") {
		s.handleCertDownload(w, r)
		return
	}

	// Default: redirect all other HTTP requests to HTTPS
	targetURL := "https://" + strings.TrimPrefix(r.URL.String(), "http://")
	http.Redirect(w, r, targetURL, http.StatusMovedPermanently)
}

func (s *ProxyServer) handlePAC(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")

	// Try reading 'pac' file from AppDataDir
	appDir, _ := config.GetAppDataDir()
	pacPath := filepath.Join(appDir, "pac")
	if _, err := os.Stat(pacPath); err == nil {
		content, err := os.ReadFile(pacPath)
		if err == nil {
			// Replace placeholders
			sContent := string(content)
			sContent = strings.ReplaceAll(sContent, "{{port}}", fmt.Sprintf("%d", s.Config.Server.Port))
			sContent = strings.ReplaceAll(sContent, "{{host}}", s.Config.Server.PACHost)
			w.Write([]byte(sContent))
			return
		}
	}

	// Fallback default
	pacContent := fmt.Sprintf(`function FindProxyForURL(url, host) { return "PROXY %s:%d"; }`, s.Config.Server.PACHost, s.Config.Server.Port)
	w.Write([]byte(pacContent))
}

func (s *ProxyServer) handleCertDownload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	w.Write(s.CA.GetRootCACertPEM())
}

func (s *ProxyServer) handleConnect(w http.ResponseWriter, r *http.Request) {
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
		port = "443"
	}

	logger.Debug("Accepted CONNECT %s from %s", r.Host, r.RemoteAddr)

	// 1. Hijack
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	// Don't close clientConn here, we hand it off.

	// 2. Respond 200 OK
	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	// 3. Bypass TLS MITM for non-HTTPS ports (e.g., SSH on port 22)
	// Also bypass if no special rules and global check is enabled
	_, hasAlter := s.Rules.GetAlterHostname(host)
	_, hasCert := s.Rules.GetCertVerify(host)
	globalVerify := true
	if b, ok := s.Config.CheckHostname.(bool); ok && !b {
		globalVerify = false
	}

	if port != "443" || (!hasAlter && !hasCert && globalVerify) {
		s.directTunnel(r, clientConn, host, port)
		return
	}

	// 4. TLS Handshake with Client (We act as Server)
	tlsConfig := &tls.Config{
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			// If SNI is missing (e.g. direct IP connection), fallback to host from CONNECT
			if hello.ServerName == "" {
				hello.ServerName = host
			}
			return s.CA.GetCertificate(hello)
		},
	}
	tlsClientConn := tls.Server(clientConn, tlsConfig)

	// Perform handshake to get SNI
	if err := tlsClientConn.Handshake(); err != nil {
		logger.Debug("TLS Handshake failed with client %s: %v", r.RemoteAddr, err)
		tlsClientConn.Close()
		return
	}
	defer tlsClientConn.Close()

	clientHelloHost := tlsClientConn.ConnectionState().ServerName
	if clientHelloHost == "" {
		clientHelloHost = host // Fallback to CONNECT host
	}

	// 5. Determine SNI to use for Remote
	targetSNI, ok := s.Rules.GetAlterHostname(clientHelloHost)
	if !ok {
		targetSNI = clientHelloHost
	}

	if targetSNI == "" {
		logger.Debug("Stripping SNI for %s", host)
	} else if targetSNI != clientHelloHost {
		logger.Debug("Replacing SNI for %s with %s", host, targetSNI)
	}

	// 6. Connect to Remote
	// Resolve IP
	clientIP, _, _ := net.SplitHostPort(r.RemoteAddr)
	remoteIP, err := s.Resolver.Resolve(r.Context(), host, net.ParseIP(clientIP))
	if err != nil {
		logger.Warn("DNS resolution failed for %s: %v", host, err)
		return
	}
	remoteAddr := net.JoinHostPort(remoteIP, port)

	// Custom Dial with explicit ServerName (SNI)
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	netConn, err := dialer.DialContext(r.Context(), "tcp", remoteAddr)
	if err != nil {
		logger.Warn("Failed to connect to remote %s (%s): %v. Invalidating DNS cache.", host, remoteAddr, err)
		s.Resolver.Invalidate(host)
		return
	}

	remoteConn := tls.Client(netConn, &tls.Config{
		ServerName:         targetSNI,
		InsecureSkipVerify: true, // We verify manually
	})

	// Perform handshake to ensure connection is good
	if err := remoteConn.Handshake(); err != nil {
		logger.Debug("Remote handshake failed for %s (SNI: %s): %v", host, targetSNI, err)
		netConn.Close()
		return
	}
	defer remoteConn.Close()

	// 7. Verify Remote Cert
	// Determine policy
	policy, ok := s.Rules.GetCertVerify(host)
	if !ok {
		policy = s.Config.CheckHostname // Default global policy
	}

	// If policy is explicitly false (bool or string "false"), skip verification
	shouldVerify := true
	switch v := policy.(type) {
	case bool:
		shouldVerify = v
	case string:
		if v == "false" {
			shouldVerify = false
		}
	}

	if shouldVerify {
		state := remoteConn.ConnectionState()
		cert := state.PeerCertificates[0]

		verified := false

		if domains, ok := policy.([]interface{}); ok {
			logger.Debug("Policy is list for %s: %v", host, domains)
			for _, d := range domains {
				if dStr, ok := d.(string); ok {
					match := tlsutil.MatchHostname(cert, dStr, true)
					logger.Debug("Checking domain %s against cert: %v", dStr, match)
					if match {
						verified = true
						break
					}
				}
			}
		} else {
			logger.Debug("Policy is not list for %s: %v", host, policy)
			if tlsutil.MatchHostname(cert, host, policy) {
				verified = true
			}
		}

		if !verified {
			logger.Warn("Certificate verification failed for %s. Cert domains: %v", host, cert.DNSNames)
			return
		}
	}

	// 8. Pipe
	logger.Info("Tunnel established: %s <-> %s (SNI: %q)", r.RemoteAddr, remoteAddr, targetSNI)

	// Ensure both are closed when the handler exits or one side finishes
	// Using a closure to copy and close allows unblocking the other side.
	go func() {
		defer remoteConn.Close()
		defer tlsClientConn.Close()
		io.Copy(remoteConn, tlsClientConn)
	}()

	// Read from remote, write to client.
	// When this finishes (remote closes or local closed via goroutine), function returns.
	// The defers at the top of handleConnect (or explicit here) should ensure cleanup.
	// We already have `defer remoteConn.Close()` (from dial) and `defer tlsClientConn.Close()` (from handshake).
	// However, to be explicit and aggressive about unblocking:
	io.Copy(tlsClientConn, remoteConn)
}

func (s *ProxyServer) directTunnel(r *http.Request, clientConn net.Conn, host, port string) {
	defer clientConn.Close()

	// 1. Resolve IP
	clientIP, _, _ := net.SplitHostPort(r.RemoteAddr)
	remoteIP, err := s.Resolver.Resolve(r.Context(), host, net.ParseIP(clientIP))
	if err != nil {
		logger.Warn("DNS resolution failed for %s: %v", host, err)
		return
	}
	remoteAddr := net.JoinHostPort(remoteIP, port)

	// 2. Dial Remote
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	remoteConn, err := dialer.DialContext(r.Context(), "tcp", remoteAddr)
	if err != nil {
		logger.Warn("Failed to connect to remote %s (%s): %v", host, remoteAddr, err)
		return
	}
	defer remoteConn.Close()

	// 3. Pipe raw data
	logger.Info("Direct tunnel established (non-HTTPS): %s <-> %s", r.RemoteAddr, remoteAddr)
	go func() {
		defer remoteConn.Close()
		defer clientConn.Close()
		io.Copy(remoteConn, clientConn)
	}()

	io.Copy(clientConn, remoteConn)
}
