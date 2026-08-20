package app

import (
	"os"
	"path/filepath"

	"github.com/xihale/snirect/config"
	"github.com/xihale/snirect/logger"
	"github.com/xihale/snirect/sysproxy"
)

// Uninstall removes the binary, configuration, and service.
func Uninstall() error {
	logger.System().Info("starting uninstall")

	if err := UninstallService(); err != nil {
		logger.System().Warn("service uninstall had errors", "error", err)
	}

	binPath := getBinPath()
	if _, err := os.Stat(binPath); err == nil {
		if err := os.Remove(binPath); err != nil {
			logger.System().Warn("failed to remove binary", "error", err)
		} else {
			logger.System().Info("removed binary", "path", binPath)
		}
	}

	logger.System().Info("clearing system proxy")
	sysproxy.ClearPAC()

	appDir, _ := config.GetAppDataDir()
	if _, err := os.Stat(appDir); err == nil {
		caCertPath := filepath.Join(appDir, "certs", "root.crt")
		if _, err := os.Stat(caCertPath); err == nil {
			logger.System().Info("removing root CA from system trust store")
			if err := sysproxy.UninstallCert(caCertPath); err != nil {
				logger.System().Warn("failed to remove certificate from system trust store", "error", err)
			}

			logger.System().Info("removing root CA from browser certificate stores")
			if err := sysproxy.UninstallFirefoxCert(); err != nil {
				logger.System().Debug("Firefox cert cleanup", "error", err)
			}
			sysproxy.UninstallChromeCert()
		}

		logger.System().Info("removing config directory", "path", appDir)
		if err := os.RemoveAll(appDir); err != nil {
			logger.System().Warn("failed to remove config directory", "error", err)
		}
	}

	removeCompletions()

	logger.System().Info("uninstall completed")
	return nil
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
			_ = os.Remove(p)
			logger.System().Debug("removed completion script", "path", p)
		}
	}
}
