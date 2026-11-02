package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// TestLoadConfig_LegacyCheckHostname pins the backward-compatible mapping of
// the old union-typed check_hostname value onto the canonical enum + allow
// list. Users' existing config files must keep working unchanged.
func TestLoadConfig_LegacyCheckHostname(t *testing.T) {
	tests := []struct {
		name      string
		toml      string
		wantMode  string
		wantAllow []string
	}{
		{"bool false", "check_hostname = false\n", "off", nil},
		{"bool true", "check_hostname = true\n", "loose", nil},
		{"string false", "check_hostname = \"false\"\n", "off", nil},
		{"string true", "check_hostname = \"true\"\n", "loose", nil},
		{"strict", "check_hostname = \"strict\"\n", "strict", nil},
		{"bare domain", "check_hostname = \"example.com\"\n", "loose", []string{"example.com"}},
		{"domain list", "check_hostname = [\"a.com\", \"b.com\"]\n", "loose", []string{"a.com", "b.com"}},
		{"canonical off", "check_hostname = \"off\"\n", "off", nil},
		{"canonical loose", "check_hostname = \"loose\"\n", "loose", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := LoadConfig(writeTempConfig(t, tc.toml))
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			if cfg.CheckHostname != tc.wantMode {
				t.Fatalf("mode = %q, want %q", cfg.CheckHostname, tc.wantMode)
			}
			if len(cfg.CheckHostnameAllow) != len(tc.wantAllow) {
				t.Fatalf("allow = %v, want %v", cfg.CheckHostnameAllow, tc.wantAllow)
			}
			for i, a := range tc.wantAllow {
				if cfg.CheckHostnameAllow[i] != a {
					t.Fatalf("allow = %v, want %v", cfg.CheckHostnameAllow, tc.wantAllow)
				}
			}
		})
	}
}

// TestLoadConfig_DefaultsSurviveUnmarshal verifies that unmarshalling a user
// file never clobbers defaults for keys the file does not set.
func TestLoadConfig_DefaultsSurviveUnmarshal(t *testing.T) {
	cfg, err := LoadConfig(writeTempConfig(t, "check_hostname = false\n"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Server.Port != 7654 {
		t.Fatalf("server.port default lost: %d", cfg.Server.Port)
	}
	if cfg.Preference.Mode != "standard" {
		t.Fatalf("preference.mode default lost: %q", cfg.Preference.Mode)
	}
}

func TestLoadConfig_RejectsRemovedFastest(t *testing.T) {
	_, err := LoadConfig(writeTempConfig(t, "preference.mode = \"fastest\"\n"))
	if err == nil {
		t.Fatal("expected error for removed fastest mode")
	}
	if !strings.Contains(err.Error(), `"fastest" was removed`) {
		t.Fatalf("error should mention removal, got: %v", err)
	}
}

// TestLoadConfig_RejectsNonStringCheckHostname pins the rejection of
// non-string check_hostname values: only bool, string, and domain lists were
// ever valid, and bools/lists are normalized, so what remains is garbage.
func TestLoadConfig_RejectsNonStringCheckHostname(t *testing.T) {
	_, err := LoadConfig(writeTempConfig(t, "check_hostname = 123\n"))
	if err == nil {
		t.Fatal("expected error for non-string check_hostname")
	}
	if !strings.Contains(err.Error(), "unsupported value") {
		t.Fatalf("error should say unsupported value, got: %v", err)
	}
}

func TestCheckHostnamePolicy(t *testing.T) {
	off := Config{CheckHostname: "off"}
	if p := off.CheckHostnamePolicy(); p.Enabled {
		t.Fatal("off should disable verification")
	}
	loose := Config{CheckHostname: "loose"}
	if p := loose.CheckHostnamePolicy(); !p.Enabled || p.Strict || len(p.Allowed) != 0 {
		t.Fatalf("loose policy wrong: %+v", p)
	}
	strict := Config{CheckHostname: "strict"}
	if p := strict.CheckHostnamePolicy(); !p.Enabled || !p.Strict {
		t.Fatalf("strict policy wrong: %+v", p)
	}
	allow := Config{CheckHostname: "off", CheckHostnameAllow: []string{"a.com"}}
	if p := allow.CheckHostnamePolicy(); !p.Enabled || len(p.Allowed) != 1 {
		t.Fatalf("allow list should override mode: %+v", p)
	}
}
