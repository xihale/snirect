//go:build windows

package sysproxy

import (
	"fmt"
	"golang.org/x/sys/windows/registry"
	"os/exec"
	"snirect/internal/logger"
)

func checkEnvPlatform(env map[string]string) {
	tools := []string{"certutil", "reg"}
	for _, tool := range tools {
		path, err := exec.LookPath(tool)
		if err == nil {
			env["Tool_"+tool] = path
		} else {
			env["Tool_"+tool] = "not found"
		}
	}
}

func installCertPlatform(certPath string) error {
	logger.Info("Attempting to install certificate: %s", certPath)

	cmd := exec.Command("certutil", "-addstore", "-user", "Root", certPath)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("failed to install certificate: %v, output: %s", err, string(output))
	}

	logger.Info("Certificate installed successfully!")
	logger.Info("Output: %s", string(output))
	return nil
}

func setPACPlatform(pacURL string) {
	logger.Info("Setting system proxy to: %s", pacURL)

	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		registry.SET_VALUE)
	if err != nil {
		logger.Warn("Failed to open registry key: %v", err)
		return
	}
	defer key.Close()

	if err := key.SetStringValue("AutoConfigURL", pacURL); err != nil {
		logger.Warn("Failed to set AutoConfigURL: %v", err)
		return
	}

	logger.Info("Proxy set successfully. You may need to restart applications for changes to take effect.")
}

func clearPACPlatform() {
	logger.Info("Clearing system proxy settings...")

	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		registry.SET_VALUE)
	if err != nil {
		logger.Debug("Failed to open registry key: %v", err)
		return
	}
	defer key.Close()

	if err := key.DeleteValue("AutoConfigURL"); err != nil {
		logger.Debug("Failed to delete AutoConfigURL: %v", err)
	}

	logger.Info("Proxy cleared successfully.")
}
