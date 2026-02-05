package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCertPolicy(t *testing.T) {
	tests := []struct {
		name    string
		data    interface{}
		want    CertPolicy
		wantErr bool
	}{
		{
			name: "bool true",
			data: true,
			want: CertPolicy{Enabled: true},
		},
		{
			name: "bool false",
			data: false,
			want: CertPolicy{Enabled: false},
		},
		{
			name: "string strict",
			data: "strict",
			want: CertPolicy{Enabled: true, Strict: true},
		},
		{
			name: "string false",
			data: "false",
			want: CertPolicy{Enabled: false},
		},
		{
			name: "string domain",
			data: "example.com",
			want: CertPolicy{Enabled: true, Allowed: []string{"example.com"}},
		},
		{
			name: "list of domains",
			data: []interface{}{"example.com", "foo.bar"},
			want: CertPolicy{Enabled: true, Allowed: []string{"example.com", "foo.bar"}},
		},
		{
			name:    "invalid type",
			data:    123,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCertPolicy(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCertPolicy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !compareCertPolicy(got, tt.want) {
				t.Errorf("ParseCertPolicy() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadConfig_Overwrite(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")
	userTOML := `
[server]
port = 9999
`
	if err := os.WriteFile(cfgPath, []byte(userTOML), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Server.Port != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.Server.Port)
	}

	if cfg.Server.Address != "127.0.0.1" {
		t.Errorf("expected default address 127.0.0.1, got %s", cfg.Server.Address)
	}

	if cfg.Timeout.Dial != 30 {
		t.Errorf("expected default dial timeout 30, got %d", cfg.Timeout.Dial)
	}
}

func TestEnsureConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	err := ensureFile(configPath, SampleConfigTOML, false)
	if err != nil {
		t.Fatalf("ensureFile failed: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}

	if string(content) != SampleConfigTOML {
		t.Errorf("generated file content does not match SampleConfigTOML")
	}
}

func compareCertPolicy(a, b CertPolicy) bool {
	if a.Enabled != b.Enabled || a.Strict != b.Strict {
		return false
	}
	if len(a.Allowed) != len(b.Allowed) {
		return false
	}
	for i := range a.Allowed {
		if a.Allowed[i] != b.Allowed[i] {
			return false
		}
	}
	return true
}

func TestRules_DollarPrefixDoesNotAffectPriority(t *testing.T) {
	rules := &Rules{
		AlterHostname: map[string]string{
			"$*google.com":   "g.cn",
			"*google.com":    "baidu.com",
			"www.google.com": "specific.com",
		},
	}
	rules.Init()

	val, _ := rules.GetAlterHostname("mail.google.com")
	if val != "baidu.com" && val != "g.cn" {
		t.Errorf("expected baidu.com or g.cn, got %s", val)
	}

	tests := []struct {
		host string
		want string
	}{
		{"www.google.com", "specific.com"},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			got, ok := rules.GetAlterHostname(tt.host)
			if !ok {
				t.Fatalf("expected match for %s", tt.host)
			}
			if got != tt.want {
				t.Errorf("GetAlterHostname(%s) = %s, want %s", tt.host, got, tt.want)
			}
		})
	}
}
