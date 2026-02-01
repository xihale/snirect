package proxy

import (
	"snirect/internal/ca"
	"snirect/internal/config"
	"snirect/internal/dns"
	"snirect/internal/logger"
	"snirect/internal/tlsutil"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
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
		Resolver: dns.NewResolver(rules),
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

	// Handle Redirects
	path := r.URL.String() // http://example.com/foo
	path = strings.TrimPrefix(path, "http://")
	
	for key, target := range s.Rules.HTTPRedirect {
		if strings.HasPrefix(path, key) {
			newURL := "https://" + target + path[len(key):]
			logger.Debug("Redirect to %s", newURL)
			http.Redirect(w, r, newURL, http.StatusMovedPermanently)
			return
		}
	}

	newURL := "https://" + path
	http.Redirect(w, r, newURL, http.StatusMovedPermanently)
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
	if port != "443" {
		s.directTunnel(r, clientConn, host, port)
		return
	}

	// 4. TLS Handshake with Client (We act as Server)
	tlsConfig := &tls.Config{
		GetCertificate: s.CA.GetCertificate,
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
	targetSNI := "" // Default to NO SNI (Matches original behavior: if not in alter_hostname, send no SNI)
	for key, val := range s.Rules.AlterHostname {
		if matched, _ := filepath.Match(key, clientHelloHost); matched {
			targetSNI = val
			break
		}
	}
	
	if targetSNI == "" {
		logger.Debug("Stripping SNI for %s", host)
	} else {
		logger.Debug("Replacing SNI for %s with %s", host, targetSNI)
	}

	// 6. Connect to Remote
	// Resolve IP
	remoteIP, err := s.Resolver.Resolve(r.Context(), host)
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
	var policy interface{} = s.Config.CheckHostname // Default global policy
	
	// Check per-domain policy in cert_verify
	// Check explicit match
	if p, ok := s.Rules.CertVerify[host]; ok {
		policy = p
	} else {
		// Check wildcard match
		for k, v := range s.Rules.CertVerify {
			if matched, _ := filepath.Match(k, host); matched {
				policy = v
				break
			}
		}
	}
	
	// If policy is explicitly false (bool), skip verification
	shouldVerify := true
	if p, ok := policy.(bool); ok && !p {
		shouldVerify = false
	}
	
	if shouldVerify {
		state := remoteConn.ConnectionState()
		cert := state.PeerCertificates[0]
		
		verified := false
		
		if domains, ok := policy.([]interface{}); ok {
			// Check if any domain in the list matches
			for _, d := range domains {
				if dStr, ok := d.(string); ok {
					if tlsutil.MatchHostname(cert, dStr, true) { 
						verified = true
						break
					}
				}
			}
		} else {
		    // No specific list, verify against original host or targetSNI?
		    if tlsutil.MatchHostname(cert, host, policy) {
		        verified = true
		    }
		}
		
		if !verified {
		    logger.Warn("Certificate verification failed for %s", host)
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
	// Function return triggers defers.
}

func (s *ProxyServer) directTunnel(r *http.Request, clientConn net.Conn, host, port string) {
	defer clientConn.Close()

	// 1. Resolve IP
	remoteIP, err := s.Resolver.Resolve(r.Context(), host)
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