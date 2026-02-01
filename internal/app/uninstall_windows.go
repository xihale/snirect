//go:build windows

package app

import (
	"os/exec"
	"snirect/internal/logger"
)

func uninstallServicePlatform() {
	taskName := "Snirect"

	logger.Info("Removing scheduled task...")

	cmd := exec.Command("schtasks", "/Delete", "/TN", taskName, "/F")
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.Debug("Failed to remove scheduled task: %v, output: %s", err, string(output))
	} else {
		logger.Info("Removed scheduled task: %s", taskName)
	}
}
