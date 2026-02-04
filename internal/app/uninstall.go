package app

import (
	"os"
	"path/filepath"
	"snirect/internal/config"
	"snirect/internal/logger"
	"snirect/internal/sysproxy"
)

// Uninstall removes the binary, configuration, and service.
func Uninstall() error {
	logger.Info("正在开始卸载...")

	uninstallServicePlatform()

	binPath := getBinPath()
	if _, err := os.Stat(binPath); err == nil {
		if err := os.Remove(binPath); err != nil {
			logger.Warn("移除二进制文件失败: %v", err)
		} else {
			logger.Info("已移除二进制文件: %s", binPath)
		}
	}

	logger.Info("正在清除系统代理设置...")
	sysproxy.ClearPAC()

	appDir, _ := config.GetAppDataDir()
	if _, err := os.Stat(appDir); err == nil {
		caCertPath := filepath.Join(appDir, "certs", "root.crt")
		if _, err := os.Stat(caCertPath); err == nil {
			logger.Info("正在从系统信任库移除根 CA...")
			if err := sysproxy.UninstallCert(caCertPath); err != nil {
				logger.Warn("从系统信任库移除证书失败: %v", err)
			}
		}

		logger.Info("正在移除配置目录: %s", appDir)
		if err := os.RemoveAll(appDir); err != nil {
			logger.Warn("移除配置目录失败: %v", err)
		}
	}

	removeCompletions()

	logger.Info("卸载完成。")
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
			os.Remove(p)
			logger.Debug("Removed completion script: %s", p)
		}
	}
}
