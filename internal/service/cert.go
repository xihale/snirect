package service

import (
	"os"
	"path/filepath"

	"github.com/xihale/snirect/internal/cert"
	"github.com/xihale/snirect/internal/config"
	"github.com/xihale/snirect/internal/logger"
	"github.com/xihale/snirect/internal/sysproxy"
)

// SetupCA initializes the CA (generating if missing) and optionally installs it to the system trust store.
func SetupCA(installToSystem bool) error {
	logger.System().Info("initializing certificate authority")

	appDir, err := config.EnsureConfig(false)
	if err != nil {
		return err
	}
	certDir := filepath.Join(appDir, "certs")
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return err
	}

	caCertPath := filepath.Join(certDir, "root.crt")
	caKeyPath := filepath.Join(certDir, "root.key")

	// Generate CA if it doesn't exist
	if _, err := cert.NewCertificateManager(caCertPath, caKeyPath); err != nil {
		return err
	}

	if installToSystem {
		logger.System().Info("installing root CA to system trust store")
		installed, err := sysproxy.InstallCert(caCertPath)
		if err != nil {
			return err
		}
		if installed {
			logger.System().Info("root CA installed")
		}

		// Also install to browser NSS databases
		if err := sysproxy.InstallFirefoxCert(caCertPath); err != nil {
			logger.System().Warn("Firefox cert install skipped", "error", err)
		}
		if err := sysproxy.InstallChromeCert(caCertPath); err != nil {
			logger.System().Warn("Chrome cert install skipped", "error", err)
		}

		logger.System().Info("restart browsers or applications to apply certificate changes")
	}

	return nil
}
