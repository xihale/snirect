//go:build darwin

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"snirect/internal/logger"
)

func uninstallServicePlatform() error {
	homeDir, _ := os.UserHomeDir()
	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.snirect.proxy.plist")

	logger.Info("Stopping and removing service...")

	cmd := exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/com.snirect.proxy", os.Getuid()))
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.Debug("launchctl bootout failed: %v, output: %s", err, string(output))
	}

	if _, err := os.Stat(plistPath); err == nil {
		if err := os.Remove(plistPath); err != nil {
			logger.Warn("Failed to remove plist file: %v", err)
		} else {
			logger.Info("Removed plist file: %s", plistPath)
		}
	}
	return nil
}
