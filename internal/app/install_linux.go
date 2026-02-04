//go:build linux

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"snirect/internal/logger"
)

func getBinPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".local", "bin", "snirect")
}

func installServicePlatform(binPath string) error {
	homeDir, _ := os.UserHomeDir()
	serviceContent := fmt.Sprintf(`[Unit]
Description=Snirect - SNI RST Bypass Proxy
After=network.target

[Service]
Type=simple
ExecStart=%s --set-proxy
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=default.target
`, binPath)

	systemdDir := filepath.Join(homeDir, ".config", "systemd", "user")
	servicePath := filepath.Join(systemdDir, "snirect.service")

	if err := os.MkdirAll(systemdDir, 0755); err != nil {
		return fmt.Errorf("创建 systemd 目录失败: %w", err)
	}

	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("写入服务文件失败: %w", err)
	}
	logger.Info("已创建 systemd 服务文件: %s", servicePath)

	runSystemctl("daemon-reload")

	logger.Info("Snirect 安装成功。")
	return nil
}

func runSystemctl(args ...string) {
	cmdArgs := append([]string{"--user"}, args...)
	cmd := exec.Command("systemctl", cmdArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.Debug("systemctl %v failed: %v, output: %s", args, err, string(output))
	}
}
