package app

import (
	"os"
	"path/filepath"
	"snirect/internal/config"
	"snirect/internal/logger"
	"snirect/internal/sysproxy"
)

// Uninstall performs the full uninstallation: service removal, binary removal, and config cleanup.
func Uninstall() {
	logger.Info("Starting uninstallation...")

	homeDir, _ := os.UserHomeDir()
	
	// 1. Stop and Disable Service
	logger.Info("Stopping service...")
	runSystemctl("stop", "snirect")
	runSystemctl("disable", "snirect")

	// 2. Remove Service File
	servicePath := filepath.Join(homeDir, ".config", "systemd", "user", "snirect.service")
	if _, err := os.Stat(servicePath); err == nil {
		if err := os.Remove(servicePath); err != nil {
			logger.Warn("Failed to remove service file: %v", err)
		} else {
			logger.Info("Removed service file: %s", servicePath)
		}
		runSystemctl("daemon-reload")
	}

	// 3. Remove Binary
	binPath := filepath.Join(homeDir, ".local", "bin", "snirect")
	if _, err := os.Stat(binPath); err == nil {
		if err := os.Remove(binPath); err != nil {
			logger.Warn("Failed to remove binary: %v", err)
		} else {
			logger.Info("Removed binary: %s", binPath)
		}
	}

	// 4. Clear System Proxy
	logger.Info("Clearing system proxy settings...")
	sysproxy.ClearPAC()

	// 5. Remove Config (Optional? Let's keep it safe or ask? Python version removed it.)
	// The prompt said "uninstall command MUST remove the binary, service, config"
	appDir, _ := config.GetAppDataDir()
	if _, err := os.Stat(appDir); err == nil {
		logger.Info("Removing configuration directory: %s", appDir)
		if err := os.RemoveAll(appDir); err != nil {
			logger.Warn("Failed to remove config dir: %v", err)
		}
	}

	// 6. Remove Completions? (Bonus)
	// We don't track where we installed them easily without logic. 
	// But we can try standard paths.
	removeCompletions(homeDir)

	logger.Info("Uninstallation complete.")
}

func removeCompletions(homeDir string) {
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
