package app

import (
	"os"
	"path/filepath"
	"snirect/internal/ca"
	"snirect/internal/config"
	"snirect/internal/logger"
	"snirect/internal/sysproxy"
)

// SetupCA initializes the CA (generating if missing) and optionally installs it to the system trust store.
func SetupCA(installToSystem bool) error {
	logger.Info("Initializing Certificate Authority...")

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
	if _, err := ca.NewCertManager(caCertPath, caKeyPath); err != nil {
		return err
	}

	if installToSystem {
		logger.Info("Installing Root CA to system trust store (requires sudo)...")
		installed, err := sysproxy.InstallCert(caCertPath)
		if err != nil {
			return err
		}
		if installed {
			logger.Info("Root CA 安装成功！")
			logger.Info("重要提示：请重启浏览器或相关应用，以使证书生效。")
		}
	}

	return nil
}
