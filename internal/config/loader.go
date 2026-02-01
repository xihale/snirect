package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type Rules struct {
	DNS           DNSConfig              `toml:"DNS"`
	HTTPRedirect  map[string]string      `toml:"http_redirect"`
	AlterHostname map[string]string      `toml:"alter_hostname"`
	CertVerify    map[string]interface{} `toml:"cert_verify"` // []string or bool
	Hosts         map[string]string      `toml:"hosts"`
}

type DNSConfig struct {
	Nameserver []string `toml:"nameserver"`
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &cfg, nil
	}
	if err != nil {
		return nil, err
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
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
