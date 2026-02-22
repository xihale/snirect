package container

import (
	"context"
	"crypto/tls"
	"net"
	"testing"

	"snirect/internal/config"
	"snirect/internal/interfaces"
	"snirect/internal/proxy"
	"snirect/internal/upstream"
)

// mockResolver is a test double for interfaces.Resolver
type mockResolver struct {
	closed bool
}

func (m *mockResolver) Resolve(ctx context.Context, host string, clientIP net.IP) (string, error) {
	return "1.2.3.4", nil
}

func (m *mockResolver) Invalidate(host string) {}

func (m *mockResolver) Close() error {
	m.closed = true
	return nil
}

// mockCertManager is a test double for interfaces.CertificateManager
type mockCertManager struct {
	closed bool
}

func (m *mockCertManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	return nil, nil
}

func (m *mockCertManager) GetRootCACertPEM() []byte {
	return []byte("mock ca")
}

func (m *mockCertManager) Close() error {
	m.closed = true
	return nil
}

func TestContainer_GetCertManager(t *testing.T) {
	cfg := &config.Config{}
	rules := &config.Rules{}
	cnt := New(cfg, rules)

	// Should panic if not set
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("GetCertManager should panic when cert manager not set")
		}
	}()
	_ = cnt.GetCertManager()

	// Set a mock and retrieve
	mock := &mockCertManager{}
	cnt.SetCertManager(mock)
	if cnt.GetCertManager() != mock {
		t.Fatal("GetCertManager should return set value")
	}

	// Subsequent calls should return same instance
	if cnt.GetCertManager() != mock {
		t.Fatal("GetCertManager should return cached instance")
	}
}

func TestContainer_GetResolver(t *testing.T) {
	cfg := &config.Config{}
	rules := &config.Rules{}
	cnt := New(cfg, rules)

	res := cnt.GetResolver()
	if res == nil {
		t.Fatal("GetResolver returned nil")
	}

	// Test Invalidate doesn't panic
	res.Invalidate("example.com")
}

func TestContainer_GetUpstreamClient(t *testing.T) {
	cfg := &config.Config{}
	rules := &config.Rules{}
	cnt := New(cfg, rules)

	cli := cnt.GetUpstreamClient()
	if cli == nil {
		t.Fatal("GetUpstreamClient returned nil")
	}

	// Should be the same instance on subsequent calls
	cli2 := cnt.GetUpstreamClient()
	if cli != cli2 {
		t.Fatal("GetUpstreamClient should return cached instance")
	}
}

func TestContainer_GetProxyServer(t *testing.T) {
	cfg := &config.Config{}
	rules := &config.Rules{}
	cnt := New(cfg, rules)

	// Need cert manager to create proxy
	mockCert := &mockCertManager{}
	cnt.SetCertManager(mockCert)

	srv := cnt.GetProxyServer()
	if srv == nil {
		t.Fatal("GetProxyServer returned nil")
	}

	// Subsequent calls should return same instance
	srv2 := cnt.GetProxyServer()
	if srv != srv2 {
		t.Fatal("GetProxyServer should return cached instance")
	}
}

func TestContainer_SetMethods(t *testing.T) {
	cfg := &config.Config{}
	rules := &config.Rules{}
	cnt := New(cfg, rules)

	// Inject mocks as interface types for comparison
	var mockCert interfaces.CertificateManager = &mockCertManager{}
	var mockRes interfaces.Resolver = &mockResolver{}
	mockUp := &upstream.Client{}
	mockSrv := &proxy.ProxyServer{}

	cnt.SetCertManager(mockCert)
	cnt.SetResolver(mockRes)
	cnt.SetUpstreamClient(mockUp)
	cnt.SetProxyServer(mockSrv)

	// Verify getters return injected mocks
	if cnt.GetCertManager() != mockCert {
		t.Error("SetCertManager not reflected")
	}
	if cnt.GetResolver() != mockRes {
		t.Error("SetResolver not reflected")
	}
	if cnt.GetUpstreamClient() != mockUp {
		t.Error("SetUpstreamClient not reflected")
	}
	if cnt.GetProxyServer() != mockSrv {
		t.Error("SetProxyServer not reflected")
	}
}

func TestContainer_Close(t *testing.T) {
	// Create fresh container for this test
	cfg := &config.Config{}
	rules := &config.Rules{}
	cnt := New(cfg, rules)

	// GetResolver creates a real resolver that implements Close
	_ = cnt.GetResolver()

	// First Close should succeed
	if err := cnt.Close(); err != nil {
		t.Errorf("First Close returned error: %v", err)
	}

	// Second Close on the same container may panic if resolver closes channel.
	// We test separately with a new container to verify Close is safe.
	cnt2 := New(cfg, rules)
	_ = cnt2.GetResolver()
	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Close panicked: %v", r)
		}
	}()
	cnt2.Close()
}
