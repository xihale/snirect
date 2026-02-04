package app

import (
	"os"
	"path/filepath"
	"snirect/internal/config"
	"snirect/internal/logger"
	"snirect/internal/sysproxy"
)

func Uninstall() {
	logger.Info("Starting uninstallation...")

	uninstallServicePlatform()

	binPath := getBinPath()
	if _, err := os.Stat(binPath); err == nil {
		if err := os.Remove(binPath); err != nil {
			logger.Warn("Failed to remove binary: %v", err)
		} else {
			logger.Info("Removed binary: %s", binPath)
		}
	}

	logger.Info("Clearing system proxy settings...")
	sysproxy.ClearPAC()

	appDir, _ := config.GetAppDataDir()
	if _, err := os.Stat(appDir); err == nil {
		caCertPath := filepath.Join(appDir, "certs", "root.crt")
		if _, err := os.Stat(caCertPath); err == nil {
			logger.Info("Removing Root CA from system trust store...")
			if err := sysproxy.UninstallCert(caCertPath); err != nil {
				logger.Warn("Failed to remove certificate from system trust store: %v", err)
			}
		}

		logger.Info("Removing configuration directory: %s", appDir)
		if err := os.RemoveAll(appDir); err != nil {
			logger.Warn("Failed to remove config dir: %v", err)
		}
	}

	removeCompletions()

	logger.Info("Uninstallation complete.")
}

func removeCompletions() {
	homeDir, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(homeDir, ".local/share/bash-completion/completions/snirect"),
		filepath.Join(homeDir, ".config/fish/completions/snirect.fish"),
		filepath.Join(homeDir, ".zfunc/_snirect"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			os.Remove(p)
			logger.Debug("Removed completion script: %s", p)
		}
	}
}
