package config

import (
	"fmt"
	"os"
	"path/filepath"
	"snirect/internal/logger"

	"github.com/pelletier/go-toml/v2"
	ruleslib "github.com/xihale/snirect-shared/rules"
)

// Rules wraps shared library's Rules for compatibility.
type Rules struct {
	*ruleslib.Rules
}

// LoadRules loads rules from a file, merging with default rules.
func LoadRules(path string) (*Rules, error) {
	defaultRules := PreparsedDefaultRules.DeepCopy()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Rules{Rules: defaultRules}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read rules file: %w", err)
	}

	userRules := ruleslib.NewRules()
	if err := userRules.FromTOML(data); err != nil {
		return nil, fmt.Errorf("failed to parse user rules: %w", err)
	}

	defaultRules.Merge(userRules)
	return &Rules{Rules: defaultRules}, nil
}

// LoadConfig loads configuration from a file.
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

// EnsureConfig ensures default config files exist.
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

// GetPath resolves a path relative to workdir.
func GetPath(workDir, filename string) string {
	return filepath.Join(workDir, filename)
}

// GetAppDataDir returns the application data directory.
func GetAppDataDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(configDir, "snirect")
	return appDir, nil
}

// ToLocalCertPolicy converts shared CertPolicy to local CertPolicy.
func ToLocalCertPolicy(sharedPolicy ruleslib.CertPolicy) CertPolicy {
	enabled := !sharedPolicy.Verify
	strict := sharedPolicy.Verify
	allowed := sharedPolicy.Allow

	if allowed == nil {
		allowed = []string{}
	}

	return CertPolicy{
		Enabled: enabled,
		Strict:  strict,
		Allowed: allowed,
	}
}

// GetCertVerify returns the cert verification policy for a host.
func (r *Rules) GetCertVerify(host string) (CertPolicy, bool) {
	if r.Rules == nil {
		return CertPolicy{}, false
	}

	sharedPolicy, ok := r.Rules.GetCertVerify(host)
	if !ok {
		return CertPolicy{}, false
	}

	return ToLocalCertPolicy(sharedPolicy), true
}

// GetAlterHostname returns the target SNI for a host.
func (r *Rules) GetAlterHostname(host string) (string, bool) {
	if r.Rules == nil {
		return "", false
	}

	return r.Rules.GetAlterHostname(host)
}

// GetHost returns the mapped IP for a host.
func (r *Rules) GetHost(host string) (string, bool) {
	if r.Rules == nil {
		return "", false
	}

	return r.Rules.GetHost(host)
}
