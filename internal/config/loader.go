package config

import (
	"fmt"
	"os"
	"path/filepath"
	"snirect/internal/util"
	"sort"

	"github.com/pelletier/go-toml/v2"
)

type Rules struct {
	AlterHostname map[string]string      `toml:"alter_hostname"`
	CertVerify    map[string]interface{} `toml:"cert_verify"` // []string or bool
	Hosts         map[string]string      `toml:"hosts"`

	alterHostnameKeys []string
	certVerifyKeys    []string
	hostsKeys         []string
}

func (r *Rules) Init() {
	r.alterHostnameKeys = getSortedKeys(r.AlterHostname)
	r.certVerifyKeys = getSortedKeysInterface(r.CertVerify)
	r.hostsKeys = getSortedKeys(r.Hosts)
}

func getSortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})
	return keys
}

func getSortedKeysInterface(m map[string]interface{}) []string {
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
		if util.MatchPattern(k, host) {
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
		if util.MatchPattern(k, host) {
			return r.Hosts[k], true
		}
	}

	return "", false
}

func (r *Rules) GetCertVerify(host string) (interface{}, bool) {
	if val, ok := r.CertVerify[host]; ok {
		return val, true
	}

	for _, k := range r.certVerifyKeys {
		if util.MatchPattern(k, host) {
			return r.CertVerify[k], true
		}
	}

	return nil, false
}

func LoadConfig(path string) (*Config, error) {
	var cfg Config

	if err := toml.Unmarshal([]byte(DefaultConfigTOML), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse default config: %w", err)
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &cfg, nil
	}
	if err != nil {
		return nil, err
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		// If user config is invalid, return the default config we already parsed above,
		// but pass the error so caller knows something went wrong.
		return &cfg, err
	}
	return &cfg, nil
}

func LoadRules(path string) (*Rules, error) {
	var rules Rules
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := toml.Unmarshal(data, &rules); err != nil {
		return nil, err
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

	if err := ensureFile(filepath.Join(appDir, "config.toml"), DefaultConfigTOML, force); err != nil {
		return "", err
	}
	if err := ensureFile(filepath.Join(appDir, "rules.toml"), DefaultRulesTOML, force); err != nil {
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
			fmt.Printf("Overwriting file: %s\n", path)
		} else {
			fmt.Printf("Creating default file: %s\n", path)
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
