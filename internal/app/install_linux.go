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

func installServicePlatform(binPath string) {
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
		logger.Fatal("创建 systemd 目录失败: %v", err)
	}

	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		logger.Fatal("写入服务文件失败: %v", err)
	}
	logger.Info("已创建 systemd 服务文件: %s", servicePath)

	runSystemctl("daemon-reload")
	runSystemctl("enable", "snirect")

	logger.Info("Snirect 安装成功并已注册（开机自启已启用）。")
}

func runSystemctl(args ...string) {
	cmdArgs := append([]string{"--user"}, args...)
	cmd := exec.Command("systemctl", cmdArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.Debug("systemctl %v failed: %v, output: %s", args, err, string(output))
	}
}
