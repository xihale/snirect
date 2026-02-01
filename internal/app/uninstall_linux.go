//go:build linux

package app

import (
	"os"
	"path/filepath"
	"snirect/internal/logger"
)

func uninstallServicePlatform() {
	homeDir, _ := os.UserHomeDir()

	logger.Info("Stopping service...")
	runSystemctl("stop", "snirect")
	runSystemctl("disable", "snirect")

	servicePath := filepath.Join(homeDir, ".config", "systemd", "user", "snirect.service")
	if _, err := os.Stat(servicePath); err == nil {
		if err := os.Remove(servicePath); err != nil {
			logger.Warn("Failed to remove service file: %v", err)
		} else {
			logger.Info("Removed service file: %s", servicePath)
		}
		runSystemctl("daemon-reload")
	}
}
