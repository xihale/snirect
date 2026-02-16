package upstream

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"snirect/internal/config"
	"snirect/internal/dns"
	"snirect/internal/logger"
	"snirect/internal/tlsutil"
)

// Client is a minimal HTTP client that routes through Snirect's internal network stack,
// applying DNS resolution, SNI rewriting, and certificate verification according to rules.
type Client struct {
	cfg      *config.Config
	rules    *config.Rules
	resolver *dns.Resolver
	timeout  time.Duration
}

// New creates a new upstream client.
func New(cfg *config.Config, rules *config.Rules) *Client {
	timeout := time.Duration(cfg.Timeout.Dial) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		cfg:      cfg,
		rules:    rules,
		resolver: dns.NewResolver(cfg, rules),
		timeout:  timeout,
	}
}

// Get performs an HTTP GET request to the given URL, routing through Snirect's internal stack.
// The request will go through DNS resolution with rule-based IP overrides, SNI modification,
// and certificate verification according to config and rules.
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	// Parse URL
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Extract host and port from URL
	host := req.URL.Hostname()
	port := req.URL.Port()
	if port == "" {
		port = "443" // Assume HTTPS for upstream requests
	}

	// Resolve IP using Snirect's resolver (applies host overrides and DNS settings)
	clientIP := net.ParseIP("127.0.0.1") // Use localhost as client IP for ECS
	remoteIP, err := c.resolver.Resolve(ctx, host, clientIP)
	if err != nil {
		return nil, fmt.Errorf("DNS resolution failed for %s: %w", host, err)
	}
	remoteAddr := net.JoinHostPort(remoteIP, port)
	logger.Debug("Upstream: resolving %s -> %s", host, remoteIP)

	// Dial TCP connection
	dialer := &net.Dialer{Timeout: c.timeout}
	netConn, err := dialer.DialContext(ctx, "tcp", remoteAddr)
	if err != nil {
		c.resolver.Invalidate(host)
		return nil, fmt.Errorf("dial failed to %s: %w", remoteAddr, err)
	}

	// Determine SNI (apply rules)
	targetSNI := c.determineSNI(host)
	logger.Debug("Upstream: SNI for %s -> %s", host, targetSNI)

	// Perform TLS handshake - force HTTP/1.1 (avoid HTTP/2 complexity)
	tlsConn := tls.Client(netConn, &tls.Config{
		ServerName:         targetSNI,
		InsecureSkipVerify: true, // We verify manually below
		NextProtos:         []string{"http/1.1"},
	})
	if err := tlsConn.Handshake(); err != nil {
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}

	// Verify certificate
	if !c.verifyCert(tlsConn, host, targetSNI) {
		state := tlsConn.ConnectionState()
		var certInfo string
		if len(state.PeerCertificates) > 0 {
			cert := state.PeerCertificates[0]
			if len(cert.DNSNames) > 0 {
				certInfo = fmt.Sprintf("server cert domains: %v", cert.DNSNames)
			} else {
				certInfo = fmt.Sprintf("server cert subject: %s", cert.Subject.CommonName)
			}
		} else {
			certInfo = "no certificate provided"
		}
		tlsConn.Close()
		return nil, fmt.Errorf("certificate verification failed for %s: %s", host, certInfo)
	}

	// Write HTTP request directly to the TLS connection
	reqStr := fmt.Sprintf("%s %s HTTP/1.1\r\nHost: %s\r\nUser-Agent: snirect-updater\r\nAccept: */*\r\nConnection: close\r\n\r\n",
		req.Method, req.URL.RequestURI(), host)
	if _, err := tlsConn.Write([]byte(reqStr)); err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Read response directly from TLS connection
	br := bufio.NewReader(tlsConn)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// The response's Body reads from the same connection; we must not close it until Body is consumed.
	// We'll wrap the Body to ensure tlsConn is closed when Body is closed.
	resp.Body = &connClosingBody{resp.Body, func() {
		tlsConn.Close()
	}}

	return resp, nil
}

// connClosingBody wraps a ReadCloser and calls closeFn when Close() is called.
type connClosingBody struct {
	io.ReadCloser
	closeFn func()
}

func (b *connClosingBody) Close() error {
	err := b.ReadCloser.Close()
	b.closeFn()
	return err
}

func (c *Client) determineSNI(host string) string {
	targetSNI, ok := c.rules.GetAlterHostname(host)
	if !ok {
		return host
	}
	if targetSNI == "" {
		// Empty string means strip SNI - but that would break TLS, so we keep original
		logger.Debug("SNI stripping rule for %s ignored (would break TLS)", host)
		return host
	}
	if targetSNI != host {
		logger.Debug("SNI replacement: %s -> %s", host, targetSNI)
	}
	return targetSNI
}

func (c *Client) verifyCert(conn *tls.Conn, host, targetSNI string) bool {
	policy, ok := c.rules.GetCertVerify(host)
	if !ok {
		policy, _ = config.ParseCertPolicy(c.cfg.CheckHostname)
	}

	// If verification is disabled, accept
	if !policy.Enabled {
		return true
	}

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return false
	}
	cert := state.PeerCertificates[0]

	// Check against allowed list if specified
	if len(policy.Allowed) > 0 {
		for _, dStr := range policy.Allowed {
			if tlsutil.MatchHostname(cert, dStr, policy) {
				return true
			}
		}
		logger.Debug("cert domains %v did not match allowed list %v", cert.DNSNames, policy.Allowed)
		return false
	}

	// Standard verification against original host
	if tlsutil.MatchHostname(cert, host, policy) {
		return true
	}

	// If SNI was altered, also check against altered SNI
	if targetSNI != "" && targetSNI != host {
		if tlsutil.MatchHostname(cert, targetSNI, policy) {
			logger.Debug("verified cert against altered SNI: %s", targetSNI)
			return true
		}
	}

	logger.Debug("hostname %s (SNI: %s) does not match cert domains %v", host, targetSNI, cert.DNSNames)
	return false
}

// DownloadFile downloads a file from the given URL to the destination path.
// It uses the internal network stack for all network operations.
func (c *Client) DownloadFile(ctx context.Context, url, destPath string) error {
	resp, err := c.Get(ctx, url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Create destination file
	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy body to file
	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
