package config

import (
	"fmt"
	"os"
	"path/filepath"
	"snirect/internal/logger"
	"sort"

	"github.com/pelletier/go-toml/v2"
)

type Rules struct {
	AlterHostname map[string]string      `toml:"alter_hostname"`
	CertVerify    map[string]interface{} `toml:"cert_verify"`
	Hosts         map[string]string      `toml:"hosts"`

	alterHostnameKeys []string
	certVerifyKeys    []string
	hostsKeys         []string
}

func (r *Rules) Init() {
	r.alterHostnameKeys = getSortedKeys(r.AlterHostname)
	r.certVerifyKeys = getSortedKeys(r.CertVerify)
	r.hostsKeys = getSortedKeys(r.Hosts)
}

func getSortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})
	return keys
}

func (r *Rules) GetAlterHostname(host string) (string, bool) {
	if val, ok := r.AlterHostname[host]; ok {
		return val, true
	}

	for _, k := range r.alterHostnameKeys {
		if MatchPattern(k, host) {
			return r.AlterHostname[k], true
		}
	}

	return "", false
}

func (r *Rules) GetHost(host string) (string, bool) {
	if val, ok := r.Hosts[host]; ok {
		return val, true
	}

	for _, k := range r.hostsKeys {
		if MatchPattern(k, host) {
			return r.Hosts[k], true
		}
	}

	return "", false
}

func (r *Rules) GetCertVerify(host string) (CertPolicy, bool) {
	if val, ok := r.CertVerify[host]; ok {
		p, _ := ParseCertPolicy(val)
		return p, true
	}

	for _, k := range r.certVerifyKeys {
		if MatchPattern(k, host) {
			p, _ := ParseCertPolicy(r.CertVerify[k])
			return p, true
		}
	}

	return CertPolicy{}, false
}

func LoadConfig(path string) (*Config, error) {
	cfg := PreparsedDefaultConfig

	if cfg.Log.File == "" {
		cfg.Log.File = GetDefaultLogPath()
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &cfg, nil
	}
	if err != nil {
		return &cfg, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return &cfg, fmt.Errorf("failed to parse user config: %w", err)
	}

	if cfg.Log.File == "" {
		cfg.Log.File = GetDefaultLogPath()
	}

	return &cfg, nil
}

func LoadRules(path string) (*Rules, error) {
	rules := PreparsedDefaultRules

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		rules.Init()
		return &rules, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read rules file: %w", err)
	}

	if err := toml.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("failed to parse user rules: %w", err)
	}

	rules.Init()
	return &rules, nil
}

func EnsureConfig(force bool) (string, error) {
	appDir, err := GetAppDataDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(appDir, 0700); err != nil {
		return "", err
	}

	if err := ensureFile(filepath.Join(appDir, "config.toml"), SampleConfigTOML, force); err != nil {
		return "", err
	}
	if err := ensureFile(filepath.Join(appDir, "rules.toml"), SampleRulesTOML, force); err != nil {
		return "", err
	}
	if err := ensureFile(filepath.Join(appDir, "pac"), DefaultPAC, force); err != nil {
		return "", err
	}

	return appDir, nil
}

func ensureFile(path, content string, force bool) error {
	if _, err := os.Stat(path); os.IsNotExist(err) || force {
		if force {
			logger.Info("Overwriting file: %s", path)
		} else {
			logger.Info("Creating default file: %s", path)
		}
		return os.WriteFile(path, []byte(content), 0644)
	} else if err != nil {
		return err
	}
	return nil
}

// Helper to resolve paths relative to executable/workdir
func GetPath(workDir, filename string) string {
	return filepath.Join(workDir, filename)
}

func GetAppDataDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(configDir, "snirect")
	return appDir, nil
}
