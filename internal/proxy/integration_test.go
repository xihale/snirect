package proxy

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/xihale/snirect-shared/rules"
	"snirect/internal/cert"
	"snirect/internal/config"
)

// TestIntegration_PAC tests that the PAC endpoint returns a valid PAC file.
func TestIntegration_PAC(t *testing.T) {
	tmpDir := t.TempDir()
	caCertPath := filepath.Join(tmpDir, "root.crt")
	caKeyPath := filepath.Join(tmpDir, "root.key")

	caMgr, err := cert.NewCertificateManager(caCertPath, caKeyPath)
	if err != nil {
		t.Fatalf("NewCertManager: %v", err)
	}
	defer caMgr.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Address: "127.0.0.1",
			Port:    0, // random
			PACHost: "127.0.0.1",
		},
	}
	rules := rules.NewRules()
	srv := NewProxyServer(cfg, &config.Rules{Rules: rules}, caMgr)

	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Server.Address, cfg.Server.Port))
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	// Run server in background
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		if err := http.Serve(ln, srv); err != nil && !errors.Is(err, http.ErrServerClosed) {
			// Ignore expected errors on shutdown
			t.Logf("http.Serve returned: %v", err)
		}
	}()

	// Ensure server is ready
	time.Sleep(100 * time.Millisecond)

	proxyURL := fmt.Sprintf("http://%s", ln.Addr().String())
	resp, err := http.Get(proxyURL + "/pac/")
	if err != nil {
		t.Fatalf("GET %s/pac/: %v", proxyURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("unexpected status: %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !contains(ct, "application/x-ns-proxy-autoconfig") {
		t.Errorf("unexpected content-type: %s", ct)
	}

	// Shutdown server
	ln.Close()
	<-serverDone
}

// TestIntegration_CertDownload tests that the root CA certificate is downloadable.
func TestIntegration_CertDownload(t *testing.T) {
	tmpDir := t.TempDir()
	caCertPath := filepath.Join(tmpDir, "root.crt")
	caKeyPath := filepath.Join(tmpDir, "root.key")

	caMgr, err := cert.NewCertificateManager(caCertPath, caKeyPath)
	if err != nil {
		t.Fatalf("NewCertManager: %v", err)
	}
	defer caMgr.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Address: "127.0.0.1",
			Port:    0,
			PACHost: "127.0.0.1",
		},
	}
	rules := rules.NewRules()
	srv := NewProxyServer(cfg, &config.Rules{Rules: rules}, caMgr)

	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Server.Address, cfg.Server.Port))
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		if err := http.Serve(ln, srv); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Logf("http.Serve returned: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond)

	proxyURL := fmt.Sprintf("http://%s", ln.Addr().String())
	resp, err := http.Get(proxyURL + "/CERT/root.crt")
	if err != nil {
		t.Fatalf("GET %s/CERT/root.crt: %v", proxyURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("unexpected status: %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/x-x509-ca-cert" {
		t.Errorf("unexpected content-type: %s", ct)
	}

	// Shutdown server
	ln.Close()
	<-serverDone
}

func contains(s, substr string) bool {
	return len(s) > 0 && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
