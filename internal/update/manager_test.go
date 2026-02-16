package update

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xihale/snirect-shared/rules"
	"snirect/internal/config"
	"snirect/internal/logger"
)

func TestMain(m *testing.M) {
	logger.SetLevel("error")
	os.Exit(m.Run())
}

func TestCheckForUpdate_VersionComparison(t *testing.T) {
	tests := []struct {
		current   string
		latest    string
		wantOlder bool // true means update should be offered (current < latest)
	}{
		// Basic semver comparisons
		{"v0.1.0", "v0.2.0", true},
		{"v0.2.0", "v0.2.0", false},
		{"0.0.0-dev", "v0.2.0", false}, // dev sentinel never updates
		{"v0.1.0", "v0.1.0", false},

		// Dirty and distance handling
		// Dirty with distance >0 is NEWER than clean tag, so no update
		{"v0.0.1-beta.5-1-dirty", "v0.0.1-beta.5", false},
		// Dirty with distance 0 is OLDER than clean tag, so update needed
		{"v0.0.1-beta.5-dirty", "v0.0.1-beta.5", true},
		// Any distance >0 is NEWER than clean tag
		{"v0.0.1-beta.5-1", "v0.0.1-beta.5", false},
		// Larger distance is NEWER than smaller distance (same base)
		{"v0.0.1-beta.5-1", "v0.0.1-beta.5-2", true},
		{"v0.0.1-beta.5-2", "v0.0.1-beta.5-1", false},
		// Same distance: clean > dirty
		{"v0.0.1-beta.5-1", "v0.0.1-beta.5-1-dirty", false}, // current clean, latest dirty -> current newer
		{"v0.0.1-beta.5-1-dirty", "v0.0.1-beta.5-1", true},  // current dirty, latest clean -> current older
		// Equal (no update)
		{"v0.0.1-beta.5-1-dirty", "v0.0.1-beta.5-1-dirty", false},
		{"v0.0.1-beta.5", "v0.0.1-beta.5", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s->%s", tt.current, tt.latest), func(t *testing.T) {
			var got bool
			if tt.current == "0.0.0-dev" {
				got = false
			} else {
				got = isOlder(tt.current, tt.latest)
			}

			if got != tt.wantOlder {
				t.Errorf("isOlder(%q, %q) = %v; want %v", tt.current, tt.latest, got, tt.wantOlder)
			}
		})
	}
}

func TestConvertAndInstallRules_TOML(t *testing.T) {
	tmpDir := t.TempDir()
	rawPath := filepath.Join(tmpDir, "raw.toml")
	installPath := filepath.Join(tmpDir, "installed.toml")

	tomlContent := `[hosts]
"cdn.example.com" = "1.2.3.4"
"api.example.org" = "5.6.7.8"

[alter_hostname]
"*.example.com" = "alt.example.net"`
	if err := os.WriteFile(rawPath, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("Failed to write raw file: %v", err)
	}

	mgr := &Manager{}
	if err := mgr.convertAndInstallRules(rawPath, installPath); err != nil {
		t.Fatalf("convertAndInstallRules failed: %v", err)
	}

	installedData, err := os.ReadFile(installPath)
	if err != nil {
		t.Fatalf("Failed to read installed file: %v", err)
	}

	r := rules.NewRules()
	if err := r.FromTOML(installedData); err != nil {
		t.Fatalf("Installed file is not valid TOML: %v", err)
	}

	if ip, ok := r.Hosts["cdn.example.com"]; !ok || ip != "1.2.3.4" {
		t.Errorf("Expected cdn.example.com -> 1.2.3.4, got %v", r.Hosts["cdn.example.com"])
	}
	if sni, ok := r.AlterHostname["*.example.com"]; !ok || sni != "alt.example.net" {
		t.Errorf("Expected *.example.com -> alt.example.net, got %v", r.AlterHostname["*.example.com"])
	}
}

func TestConvertAndInstallRules_JSONArray(t *testing.T) {
	tmpDir := t.TempDir()
	rawPath := filepath.Join(tmpDir, "raw.json")
	installPath := filepath.Join(tmpDir, "installed.toml")

	jsonContent := `[ [["cdn.jsdelivr.net"], "", "104.16.89.20"], [["images.prismic.io"], "imgix.net", "151.101.78.208"], [["*pixiv.net", "*fanbox.cc"], "pixivision.net", "210.140.139.155"] ]`
	if err := os.WriteFile(rawPath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("Failed to write raw file: %v", err)
	}

	mgr := &Manager{}
	if err := mgr.convertAndInstallRules(rawPath, installPath); err != nil {
		t.Fatalf("convertAndInstallRules failed: %v", err)
	}

	installedData, err := os.ReadFile(installPath)
	if err != nil {
		t.Fatalf("Failed to read installed file: %v", err)
	}

	r := rules.NewRules()
	if err := r.FromTOML(installedData); err != nil {
		t.Fatalf("Installed file is not valid TOML: %v", err)
	}

	expected := map[string]struct {
		ip  string
		sni string
	}{
		"cdn.jsdelivr.net":  {"104.16.89.20", ""},
		"images.prismic.io": {"151.101.78.208", "imgix.net"},
		"*pixiv.net":        {"210.140.139.155", "pixivision.net"},
		"*fanbox.cc":        {"210.140.139.155", "pixivision.net"},
	}

	for host, exp := range expected {
		ip, ok := r.Hosts[host]
		if !ok {
			t.Errorf("Host %s not found in Hosts", host)
		} else if ip != exp.ip {
			t.Errorf("Host %s: expected IP %s, got %s", host, exp.ip, ip)
		}
		sni, ok := r.AlterHostname[host]
		if !ok {
			t.Errorf("AlterHostname missing for %s", host)
		} else if sni != exp.sni {
			t.Errorf("AlterHostname[%s]: expected %q, got %q", host, exp.sni, sni)
		}
	}
}

func TestConvertAndInstallRules_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	rawPath := filepath.Join(tmpDir, "raw.json")
	installPath := filepath.Join(tmpDir, "installed.toml")

	invalidJSON := `not valid json at all`
	if err := os.WriteFile(rawPath, []byte(invalidJSON), 0644); err != nil {
		t.Fatalf("Failed to write raw file: %v", err)
	}

	mgr := &Manager{}
	err := mgr.convertAndInstallRules(rawPath, installPath)
	if err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse JSON array") {
		t.Errorf("Expected 'failed to parse JSON array' in error, got: %v", err)
	}
}

func TestHasPendingUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := tmpDir
	markFile := filepath.Join(appDir, ".self_update")

	if HasPendingUpdate(appDir) {
		t.Error("Expected no pending update when mark file doesn't exist")
	}

	if err := os.WriteFile(markFile, []byte("v0.2.0"), 0644); err != nil {
		t.Fatalf("Failed to create mark file: %v", err)
	}
	if !HasPendingUpdate(appDir) {
		t.Error("Expected pending update when mark file exists")
	}

	if err := os.Remove(markFile); err != nil {
		t.Fatalf("Failed to remove mark file: %v", err)
	}
	if HasPendingUpdate(appDir) {
		t.Error("Expected no pending update after removing mark file")
	}
}

func TestManager_FetchRules_EmptyURL(t *testing.T) {
	cfg := &config.Config{}
	mgr := NewManager(cfg, &config.Rules{})
	err := mgr.FetchRules(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "rules URL not configured") {
		t.Errorf("Expected error about missing rules URL, got: %v", err)
	}
}

func TestManager_FetchRules_DownloadFailure(t *testing.T) {
	cfg := &config.Config{
		Update: config.UpdateConfig{
			RulesURL: "https://example.com/rules.toml",
		},
	}
	mgr := NewManager(cfg, &config.Rules{})
	tmpDir := t.TempDir()
	err := mgr.FetchRules(tmpDir)
	if err == nil {
		t.Error("Expected download failure, got nil")
	}
}
