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

	// Direct unmarshal into the default config; fields not present remain as defaults.
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return &cfg, fmt.Errorf("failed to parse user config: %w", err)
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
