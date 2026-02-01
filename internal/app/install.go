package app

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"snirect/internal/logger"
)

// Install performs the full installation: binary copy, CA setup, and Systemd registration.
func Install() {
	homeDir, _ := os.UserHomeDir()
	binDir := filepath.Join(homeDir, ".local", "bin")
	targetBinPath := filepath.Join(binDir, "snirect")

	// 1. Install Binary
	logger.Info("Installing binary to %s...", targetBinPath)
	if err := os.MkdirAll(binDir, 0755); err != nil {
		logger.Fatal("Failed to create bin dir: %v", err)
	}

	srcPath, err := os.Executable()
	if err != nil {
		logger.Fatal("Failed to get executable path: %v", err)
	}

	if err := copyFile(srcPath, targetBinPath); err != nil {
		logger.Fatal("Failed to copy binary: %v", err)
	}
	if err := os.Chmod(targetBinPath, 0755); err != nil {
		logger.Fatal("Failed to set binary permissions: %v", err)
	}

	// 2. Initialize CA and Install Certificate
	if err := SetupCA(true); err != nil {
		logger.Warn("Certificate setup warning: %v. You may need to run 'snirect install-cert' manually.", err)
	}

	// 3. Install Systemd Service
	installSystemdService(homeDir, targetBinPath)
}

func installSystemdService(homeDir, binPath string) {
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

	// 3. Reload and Enable
	runSystemctl("daemon-reload")
	runSystemctl("enable", "snirect")
	
	logger.Info("Snirect installed and registered (auto-start enabled).")
	fmt.Printf("\nSuccess! Make sure %s is in your PATH.\n", filepath.Dir(binPath))
	fmt.Println("To start the service now, run: systemctl --user start snirect")
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	return err
}

func runSystemctl(args ...string) {
	cmdArgs := append([]string{"--user"}, args...)
	cmd := exec.Command("systemctl", cmdArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.Debug("systemctl %v failed: %v, output: %s", args, err, string(output))
	}
}
