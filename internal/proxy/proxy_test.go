package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/xihale/snirect-shared/rules"
	"snirect/internal/cert"
	"snirect/internal/config"
	"snirect/internal/dns"
)

// mockConn 是用于测试的 net.Conn 包装器
type mockConn struct {
	net.Conn
	closeWriteErr error
}

func (m *mockConn) CloseWrite() error {
	// 模拟 CloseWrite，如果实现了接口就调用
	if cw, ok := m.Conn.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}
	// net.Pipe 不支持 CloseWrite，忽略
	return m.closeWriteErr
}

// TestTunnel 测试基本的双向隧道功能
func TestTunnel(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	ps := &ProxyServer{
		Config: &config.Config{
			Server: config.ServerConfig{
				BufferSize: 65536, // default 64KB
			},
		},
	}

	done := make(chan struct{})
	go func() {
		ps.tunnel(c1, c2)
		close(done)
	}()

	// 发送测试数据
	msg1 := []byte("hello from c1 to c2")
	msg2 := []byte("hello from c2 to c1")

	if _, err := c1.Write(msg1); err != nil {
		t.Fatalf("c1 write failed: %v", err)
	}
	if _, err := c2.Write(msg2); err != nil {
		t.Fatalf("c2 write failed: %v", err)
	}

	// 验证双向接收
	c2Recv := make([]byte, len(msg1))
	n, err := c2.Read(c2Recv)
	if err != nil && err != io.EOF {
		t.Fatalf("c2 read failed: %v", err)
	}
	if n != len(msg1) {
		t.Fatalf("c2 received %d bytes, want %d", n, len(msg1))
	}
	if string(c2Recv[:n]) != string(msg1) {
		t.Fatalf("c2 received %q, want %q", c2Recv[:n], msg1)
	}

	c1Recv := make([]byte, len(msg2))
	n, err = c1.Read(c1Recv)
	if err != nil && err != io.EOF {
		t.Fatalf("c1 read failed: %v", err)
	}
	if n != len(msg2) {
		t.Fatalf("c1 received %d bytes, want %d", n, len(msg2))
	}
	if string(c1Recv[:n]) != string(msg2) {
		t.Fatalf("c1 received %q, want %q", c1Recv[:n], msg2)
	}

	// 关闭一侧，确认 tunnel 正确退出
	c1.Close()
	select {
	case <-done:
		// tunnel 正常退出
	case <-time.After(5 * time.Second):
		t.Fatal("tunnel did not exit within 5 seconds after closing c1")
	}
}

// TestTunnel_SingleSideClose 测试一侧关闭时隧道的行为
func TestTunnel_SingleSideClose(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	ps := &ProxyServer{
		Config: &config.Config{
			Server: config.ServerConfig{
				BufferSize: 65536,
			},
		},
	}

	done := make(chan struct{})
	go func() {
		ps.tunnel(c1, c2)
		close(done)
	}()

	// 只从一侧写入数据，然后关闭该侧
	c1.Write([]byte("test data"))
	time.Sleep(100 * time.Millisecond) // 给时间让数据传递

	// 关闭 c1，tunnel 应该退出
	c1.Close()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("tunnel did not exit after c1 close")
	}
}

// TestTunnel_ContextCancellation 测试 context 取消
func TestTunnel_ContextCancellation(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	// 使用自定义 context 覆盖 tunnel 的默认 5 分钟超时
	// 但 tunnel 内部使用固定的 5 分钟超时，无法直接测试上下文取消
	// 这是一个已知限制；为了测试，我们需要修改 tunnel 支持外部 context
	// 暂时跳过，后续 Task 2.3 可能会定义接口改进
	t.Skip("requires external context injection (future enhancement)")
}

// TestTunnel_GoroutineLeak 测试 tunnel 不会泄漏 goroutine
func TestTunnel_GoroutineLeak(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	ps := &ProxyServer{
		Config: &config.Config{
			Server: config.ServerConfig{
				BufferSize: 65536,
			},
		},
	}

	before := runtime.NumGoroutine()

	done := make(chan struct{})
	go func() {
		ps.tunnel(c1, c2)
		close(done)
	}()

	// 快速关闭一侧以触发退出
	c1.Close()
	<-done

	// 检查 goroutine 数量是否恢复
	after := runtime.NumGoroutine()
	if after > before+2 { // 允许少量临时增长
		t.Fatalf("possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// mockCertificateManager is a test double for interfaces.CertificateManager.
type mockCertificateManager struct{}

func (m *mockCertificateManager) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return nil, nil
}
func (m *mockCertificateManager) GetRootCACertPEM() []byte {
	return []byte("-----BEGIN CERTIFICATE-----\nFAKE\n-----END CERTIFICATE-----")
}
func (m *mockCertificateManager) Close() error { return nil }

// TestHandleHTTP_PAC tests that the PAC endpoint returns correct JavaScript.
// Skipped due to dependency on existing PAC file in user config directory.
func TestHandleHTTP_PAC(t *testing.T) {
	t.Skip("requires isolated config directory; to be revisited")
}

// TestHandleHTTP_CertDownload tests that the CERT endpoint returns the root CA PEM.
func TestHandleHTTP_CertDownload(t *testing.T) {
	ps := &ProxyServer{
		CA: &mockCertificateManager{},
	}

	req := httptest.NewRequest("GET", "/CERT/root.crt", nil)
	rr := httptest.NewRecorder()
	ps.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "BEGIN CERTIFICATE") {
		t.Error("response body does not contain certificate PEM")
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "application/x-x509-ca-cert" {
		t.Errorf("Content-Type: got %q, want application/x-x509-ca-cert", ct)
	}
}

// TestHandleHTTP_Redirect tests that non-PAC/CERT requests redirect to HTTPS.
func TestHandleHTTP_Redirect(t *testing.T) {
	ps := &ProxyServer{
		Config: &config.Config{
			Server: config.ServerConfig{
				Port: 7654,
			},
		},
		CA: &mockCertificateManager{},
	}

	req := httptest.NewRequest("GET", "http://example.com/foo", nil)
	rr := httptest.NewRecorder()
	ps.ServeHTTP(rr, req)

	// Expect 301 Moved Permanently to https://example.com/foo
	if rr.Code != http.StatusMovedPermanently {
		t.Fatalf("expected status 301, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	expected := "https://example.com/foo"
	if loc != expected {
		t.Fatalf("Location: got %q, want %q", loc, expected)
	}
}

// TestShouldIntercept tests the logic of deciding whether to MITM.
func TestShouldIntercept(t *testing.T) {
	baseRules := rules.NewRules()
	// Helper to create ProxyServer with given settings.
	makePS := func(checkHostname interface{}, hasAlter bool) *ProxyServer {
		rls := &config.Rules{Rules: baseRules}
		if hasAlter {
			baseRules.AlterHostname["example.com"] = "fake.com"
		}
		return &ProxyServer{
			Config: &config.Config{CheckHostname: checkHostname},
			Rules:  rls,
		}
	}

	tests := []struct {
		name          string
		port          string
		checkHostname interface{}
		hasAlter      bool
		want          bool
	}{
		// Port not 443 -> never intercept
		{"port 80", "80", true, false, false},
		// Port 443, no alter, global verification enabled (policy.Enabled true) -> direct tunnel (no intercept)
		{"no alter, verif enabled", "443", true, false, false},
		// Port 443, no alter, global verification disabled (policy.Enabled false) -> intercept to bypass verification
		{"no alter, verif disabled", "443", false, false, true},
		// Port 443, alter rule present -> intercept
		{"alter rule", "443", true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := makePS(tt.checkHostname, tt.hasAlter)
			got := ps.shouldIntercept("example.com", tt.port)
			if got != tt.want {
				t.Fatalf("shouldIntercept: got %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDetermineSNI tests SNI determination based on rules.
func TestDetermineSNI(t *testing.T) {
	baseRules := rules.NewRules()
	ps := &ProxyServer{
		Config: &config.Config{},
		Rules:  &config.Rules{Rules: baseRules},
	}

	// No rule -> returns original host
	if sn := ps.determineSNI("example.com", "example.com"); sn != "example.com" {
		t.Errorf("no rule: got %s, want example.com", sn)
	}

	// Alter rule to change SNI
	baseRules.AlterHostname["example.com"] = "changed.com"
	if sn := ps.determineSNI("example.com", "example.com"); sn != "changed.com" {
		t.Errorf("replacement: got %s, want changed.com", sn)
	}

	// Alter rule to empty string strips SNI -> returns empty string
	baseRules.AlterHostname["example.com"] = ""
	if sn := ps.determineSNI("example.com", "example.com"); sn != "" {
		t.Errorf("strip: got %q, want empty string", sn)
	}
}

// mockResolver is a test double for interfaces.Resolver.
type mockResolver struct {
	resolveFunc    func(ctx context.Context, host string, clientIP net.IP) (string, error)
	invalidateFunc func(host string)
}

func (m *mockResolver) Resolve(ctx context.Context, host string, clientIP net.IP) (string, error) {
	return m.resolveFunc(ctx, host, clientIP)
}

func (m *mockResolver) Invalidate(host string) {
	if m.invalidateFunc != nil {
		m.invalidateFunc(host)
	}
}

func (m *mockResolver) Close() error { return nil }

// TestDirectTunnel_DNSFailure tests that directTunnel handles DNS resolution failure gracefully.
func TestDirectTunnel_DNSFailure(t *testing.T) {
	mock := &mockResolver{
		resolveFunc: func(ctx context.Context, host string, clientIP net.IP) (string, error) {
			return "", fmt.Errorf("DNS lookup failed")
		},
	}
	ps := &ProxyServer{
		Config:   &config.Config{},
		Resolver: mock,
	}
	c1, c2 := net.Pipe()
	// directTunnel should call c1.Close() and return without panicking.
	ps.directTunnel(context.Background(), c1, "example.com", "443")
	// c1 should be closed
	_, err := c1.Write([]byte("test"))
	if err == nil {
		t.Error("expected clientConn to be closed")
	}
	c2.Close()
}

// TestDirectTunnel_ConnectFailure tests that directTunnel handles connection failure gracefully.
func TestDirectTunnel_ConnectFailure(t *testing.T) {
	// Resolver returns an IP that will refuse connection (non-listening)
	mock := &mockResolver{
		resolveFunc: func(ctx context.Context, host string, clientIP net.IP) (string, error) {
			return "127.0.0.1", nil // but we will dial a port that is not listening
		},
	}
	ps := &ProxyServer{
		Config: &config.Config{
			Timeout: config.TimeoutConfig{Dial: 1},
		},
		Resolver: mock,
	}
	c1, _ := net.Pipe()
	// Use a port that is unlikely to be listening
	ps.directTunnel(context.Background(), c1, "example.com", "9") // port 9 is typically unused
	// Should attempt dial, fail, close c1
	_, err := c1.Write([]byte("test"))
	if err == nil {
		t.Error("expected clientConn to be closed after dial failure")
	}
}

// TestConnectToRemote_DNSFailure tests that connectToRemote returns error when DNS resolution fails.
func TestConnectToRemote_DNSFailure(t *testing.T) {
	mock := &mockResolver{
		resolveFunc: func(ctx context.Context, host string, clientIP net.IP) (string, error) {
			return "", fmt.Errorf("DNS resolution failed")
		},
	}
	ps := &ProxyServer{
		Config:   &config.Config{},
		Resolver: mock,
	}
	_, err := ps.connectToRemote(context.Background(), "example.com", "443", "192.168.1.1", "example.com")
	if err == nil {
		t.Fatal("expected DNS error")
	}
	if !strings.Contains(err.Error(), "DNS resolution failed") {
		t.Errorf("error %q doesn't contain DNS", err.Error())
	}
}

// TestConnectToRemote_DialFailure tests that connectToRemote returns error when dialing fails.
func TestConnectToRemote_DialFailure(t *testing.T) {
	mock := &mockResolver{
		resolveFunc: func(ctx context.Context, host string, clientIP net.IP) (string, error) {
			return "127.0.0.1", nil // resolve to loopback but use an unused port
		},
	}
	ps := &ProxyServer{
		Config: &config.Config{
			Timeout: config.TimeoutConfig{Dial: 1},
		},
		Resolver: mock,
	}
	// Port 9 is typically unused and will cause connection refused.
	_, err := ps.connectToRemote(context.Background(), "example.com", "9", "192.168.1.1", "example.com")
	if err == nil {
		t.Fatal("expected dial failure")
	}
	if !strings.Contains(err.Error(), "dial failed") {
		t.Errorf("error %q doesn't indicate dial failure", err.Error())
	}
}

// TestConnectToRemote_TLSFailure tests that connectToRemote returns error when TLS handshake fails.
func TestConnectToRemote_TLSFailure(t *testing.T) {
	// Start a raw TCP listener that accepts and immediately closes.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())

	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			conn.Close()
		}
	}()

	mock := &mockResolver{
		resolveFunc: func(ctx context.Context, host string, clientIP net.IP) (string, error) {
			return "127.0.0.1", nil
		},
	}
	ps := &ProxyServer{
		Config: &config.Config{
			Timeout: config.TimeoutConfig{Dial: 5},
		},
		Resolver: mock,
	}

	_, err = ps.connectToRemote(context.Background(), "example.com", port, "192.168.1.1", "example.com")
	if err == nil {
		t.Fatal("expected TLS handshake failure")
	}
}

// nonHijacker is a http.ResponseWriter that does NOT implement Hijacker.
type nonHijacker struct{}

func (n *nonHijacker) Header() http.Header {
	return http.Header{}
}

// TestProxy_HTTPS_CONNECT is an integration test that performs a full HTTPS CONNECT through the proxy.
func TestProxy_HTTPS_CONNECT(t *testing.T) {
	// Setup CA
	tmpDir := t.TempDir()
	caCertPath := filepath.Join(tmpDir, "root.crt")
	caKeyPath := filepath.Join(tmpDir, "root.key")
	certMgr, err := cert.NewCertificateManager(caCertPath, caKeyPath)
	if err != nil {
		t.Fatalf("NewCertManager failed: %v", err)
	}
	defer certMgr.Close()

	// Create rules: map target.example to 127.0.0.1 (loopback)
	sharedRules := rules.NewRules()
	sharedRules.Hosts["target.example"] = "127.0.0.1"
	cfgRules := &config.Rules{Rules: sharedRules}

	// Proxy configuration: disable hostname verification to trigger MITM
	cfg := &config.Config{
		CheckHostname: false,
		Server: config.ServerConfig{
			Address:    "127.0.0.1",
			Port:       50003, // fixed port to avoid race on dynamic port assignment
			BufferSize: 65536,
		},
	}

	// Create resolver and proxy server
	resolver := dns.NewResolver(cfg, cfgRules)
	srv := NewProxyServerWithResolver(cfg, cfgRules, certMgr, resolver)

	// Start the proxy in a goroutine
	go func() {
		if err := srv.Start(); err != nil && !strings.Contains(err.Error(), "Server closed") {
			t.Errorf("Proxy server error: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond)

	proxyPort := cfg.Server.Port
	if proxyPort == 0 {
		t.Fatal("proxy did not start on a port")
	}
	t.Logf("Proxy listening on port %d", proxyPort)

	// Start a test HTTPS server that will be the upstream
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	}))
	defer ts.Close()

	tsURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	upstreamPort := tsURL.Port()
	targetHost := "target.example"

	// HTTP client using the proxy, skipping TLS verification of the proxy's cert
	proxyURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", proxyPort))
	if err != nil {
		t.Fatalf("parse proxy URL: %v", err)
	}
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	reqURL := fmt.Sprintf("https://%s:%s/", targetHost, upstreamPort)
	t.Logf("Requesting %s", reqURL)
	resp, err := client.Get(reqURL)
	if err != nil {
		t.Fatalf("HTTP request through proxy failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected HTTP status: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	if string(body) != "OK" {
		t.Errorf("unexpected response body: %s", body)
	}
}
func (n *nonHijacker) Write([]byte) (int, error) { return 0, nil }
func (n *nonHijacker) WriteHeader(int)           {}

// TestHijackConnection_Error tests that hijackConnection returns an error when the ResponseWriter is not a Hijacker.
func TestHijackConnection_Error(t *testing.T) {
	ps := &ProxyServer{}
	w := &nonHijacker{}
	_, err := ps.hijackConnection(w)
	if err == nil {
		t.Error("expected error from non-hijack writer")
	}
}

// TestHandshakeClient_Error tests that handshakeClient fails when the clientConn does not perform a proper TLS handshake.
func TestHandshakeClient_Error(t *testing.T) {
	ps := &ProxyServer{
		CA: &mockCertificateManager{},
	}
	// Use a pipe; one side will act as clientConn but we send invalid data
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	// Send garbage to cause handshake failure
	go func() {
		clientConn.Write([]byte("invalid TLS"))
		clientConn.Close()
	}()

	_, _, err := ps.handshakeClient(serverConn, "example.com")
	if err == nil {
		t.Error("expected handshake error")
	}
}

// TestHandleHTTP_PAC_WithFile tests the PAC endpoint when a custom pac file exists in the config directory.
func TestHandleHTTP_PAC_WithFile(t *testing.T) {
	// Save and restore XDG_CONFIG_HOME to isolate test environment
	origEnv := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", origEnv)

	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)

	// Create app directory: $XDG_CONFIG_HOME/snirect
	appDir := filepath.Join(tmp, "snirect")
	if err := os.MkdirAll(appDir, 0700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	// Write a custom PAC file
	pacPath := filepath.Join(appDir, "pac")
	pacContent := "function FindProxyForURL(url, host) { return \"PROXY custom.com:9999\"; }"
	if err := os.WriteFile(pacPath, []byte(pacContent), 0644); err != nil {
		t.Fatalf("write pac failed: %v", err)
	}

	ps := &ProxyServer{
		Config: &config.Config{
			Server: config.ServerConfig{
				PACHost: "custom.com",
				Port:    9999,
			},
		},
		CA: &mockCertificateManager{},
	}

	req := httptest.NewRequest("GET", "/pac/anything", nil)
	rr := httptest.NewRecorder()
	ps.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "PROXY custom.com:9999") {
		t.Errorf("PAC content unexpected: %s", rr.Body.String())
	}
	if !strings.Contains(rr.Header().Get("Content-Type"), "application/x-ns-proxy-autoconfig") {
		t.Errorf("Content-Type: got %q, want application/x-ns-proxy-autoconfig", rr.Header().Get("Content-Type"))
	}
}
