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
		logger.Fatal("Failed to create systemd directory: %v", err)
	}

	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		logger.Fatal("Failed to write service file: %v", err)
	}
	logger.Info("Created systemd service file at: %s", servicePath)

	runSystemctl("daemon-reload")
	runSystemctl("enable", "snirect")

	logger.Info("Snirect installed and registered (auto-start enabled).")
	logger.Info("Service file: %s", servicePath)
}

func runSystemctl(args ...string) {
	cmdArgs := append([]string{"--user"}, args...)
	cmd := exec.Command("systemctl", cmdArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.Debug("systemctl %v failed: %v, output: %s", args, err, string(output))
	}
}
