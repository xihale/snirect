package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"github.com/xihale/snirect/internal/logger"
)

// LoadConfig loads configuration from a file.
func LoadConfig(path string) (*Config, error) {
	cfg := PreparsedDefaultConfig

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &cfg, nil
	}
	if err != nil {
		return &cfg, fmt.Errorf("failed to read config file: %w", err)
	}

	data, err = normalizeLegacyTOML(data)
	if err != nil {
		return &cfg, err
	}

	// Direct unmarshal into the default config; fields not present remain as defaults.
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return &cfg, fmt.Errorf("failed to parse user config: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return &cfg, err
	}

	return &cfg, nil
}

// normalizeLegacyTOML rewrites deprecated check_hostname shapes onto the
// canonical keys so the Config struct can stay typed. The old value was a
// union: bool, the strings "false"/"true"/"strict", a bare domain string, or
// a domain list. Mapping:
//
//	false / "false"           -> check_hostname = "off"
//	true / "true"             -> check_hostname = "loose"
//	"strict"                  -> kept
//	bare domain / domain list -> check_hostname_allow
func normalizeLegacyTOML(data []byte) ([]byte, error) {
	var doc map[string]interface{}
	if err := toml.Unmarshal(data, &doc); err != nil {
		return data, err
	}
	raw, ok := doc["check_hostname"]
	if !ok {
		return data, nil
	}
	switch v := raw.(type) {
	case bool:
		if v {
			doc["check_hostname"] = "loose"
		} else {
			doc["check_hostname"] = "off"
		}
	case string:
		switch v {
		case "off", "loose", "strict": // canonical already
		case "false":
			doc["check_hostname"] = "off"
		case "true":
			doc["check_hostname"] = "loose"
		default:
			// Bare domain: legacy single-entry allow list.
			delete(doc, "check_hostname")
			doc["check_hostname_allow"] = []string{v}
		}
	case []interface{}:
		allow := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("check_hostname: expected domain strings, got %T", item)
			}
			allow = append(allow, s)
		}
		delete(doc, "check_hostname")
		doc["check_hostname_allow"] = allow
	default:
		return nil, fmt.Errorf("check_hostname: unsupported value %v (type %T); use \"off\", \"loose\", \"strict\", or check_hostname_allow = [ ... ]", raw, raw)
	}
	return toml.Marshal(doc)
}

// validate rejects removed or misspelled values. It runs only on
// file-loaded configs; check_hostname shapes are fully normalized by
// normalizeLegacyTOML before this point. Programmatically built Configs
// rely on the zero-tolerant default branches in CheckHostnamePolicy and
// the resolver.
func validate(cfg *Config) error {
	switch cfg.Preference.Mode {
	case "", IPPreferenceStandard, IPPreferenceIPv6, IPPreferenceIPv4:
	default:
		if cfg.Preference.Mode == IPPreferenceMode("fastest") {
			return fmt.Errorf("preference.mode: \"fastest\" was removed; use \"standard\"")
		}
		return fmt.Errorf("preference.mode: %q is not one of \"standard\", \"ipv6\", \"ipv4\"", cfg.Preference.Mode)
	}
	return nil
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
	if err := ensureFile(filepath.Join(appDir, "pac"), DefaultPAC, force); err != nil {
		return "", err
	}

	return appDir, nil
}

func ensureFile(path, content string, force bool) error {
	if _, err := os.Stat(path); os.IsNotExist(err) || force {
		if force {
			logger.System().Info("overwriting file", "path", path)
		} else {
			logger.System().Info("creating default file", "path", path)
		}
		return os.WriteFile(path, []byte(content), 0644)
	} else if err != nil {
		return err
	}
	return nil
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
