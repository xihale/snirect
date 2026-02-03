//go:build windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"snirect/internal/logger"
)

func getBinPath() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		homeDir, _ := os.UserHomeDir()
		localAppData = filepath.Join(homeDir, "AppData", "Local")
	}
	return filepath.Join(localAppData, "Programs", "snirect", "snirect.exe")
}

func installServicePlatform(binPath string) {
	taskName := "Snirect"

	cmd := exec.Command("schtasks", "/Create", "/TN", taskName, "/TR", fmt.Sprintf(`"%s" --set-proxy`, binPath),
		"/SC", "ONLOGON", "/RL", "HIGHEST", "/F")

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warn("Failed to create scheduled task: %v, output: %s", err, string(output))
		logger.Info("You can manually create a startup task or run snirect at login.")
		return
	}

	logger.Info("Created scheduled task: %s", taskName)
	logger.Info("Snirect will start automatically on login.")
}
