package config

import (
	"fmt"
	"os"
	"path/filepath"
	"snirect/internal/logger"
	"sort"
	"strings"

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
	r.AlterHostname = normalizeMap(r.AlterHostname)
	r.CertVerify = normalizeMap(r.CertVerify)
	r.Hosts = normalizeMap(r.Hosts)

	r.alterHostnameKeys = getSortedKeys(r.AlterHostname)
	r.certVerifyKeys = getSortedKeys(r.CertVerify)
	r.hostsKeys = getSortedKeys(r.Hosts)
}

func normalizeMap[T any](m map[string]T) map[string]T {
	if m == nil {
		return nil
	}
	newM := make(map[string]T, len(m))
	for k, v := range m {
		newK := strings.TrimPrefix(k, "$")
		newM[newK] = v
	}
	return newM
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

func (r *Rules) DeepCopy() Rules {
	newR := *r
	newR.AlterHostname = copyMap(r.AlterHostname)
	newR.CertVerify = copyMap(r.CertVerify)
	newR.Hosts = copyMap(r.Hosts)
	return newR
}

func copyMap[T any](m map[string]T) map[string]T {
	if m == nil {
		return nil
	}
	newM := make(map[string]T, len(m))
	for k, v := range m {
		newM[k] = v
	}
	return newM
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
	rules := PreparsedDefaultRules.DeepCopy()
	rules.Init()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &rules, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read rules file: %w", err)
	}

	var userRules Rules
	if err := toml.Unmarshal(data, &userRules); err != nil {
		return nil, fmt.Errorf("failed to parse user rules: %w", err)
	}
	userRules.Init()

	rules.AlterHostname = mergeMap(rules.AlterHostname, userRules.AlterHostname)
	rules.CertVerify = mergeMap(rules.CertVerify, userRules.CertVerify)
	rules.Hosts = mergeMap(rules.Hosts, userRules.Hosts)

	rules.Init()
	return &rules, nil
}

func mergeMap[T any](dst, src map[string]T) map[string]T {
	if dst == nil {
		dst = make(map[string]T)
	}
	for k, v := range src {
		dst[k] = v
	}
	return dst
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
