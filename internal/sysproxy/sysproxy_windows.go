//go:build windows

package sysproxy

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/sys/windows/registry"
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

	certData, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("failed to read certificate: %v", err)
	}

	if isCertInstalled(certData) {
		logger.Info("Certificate already installed in system trust store")
		return nil
	}

	cmd := exec.Command("certutil", "-addstore", "-user", "Root", certPath)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("failed to install certificate: %v, output: %s", err, string(output))
	}

	logger.Info("Certificate installed successfully!")
	logger.Info("Output: %s", string(output))
	return nil
}

func isCertInstalled(certData []byte) bool {
	cmd := exec.Command("certutil", "-user", "-verifystore", "Root", "Snirect Root CA")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "Snirect Root CA") {
		return false
	}

	cmd = exec.Command("certutil", "-user", "-store", "Root")
	output, err = cmd.Output()
	if err != nil {
		return false
	}

	return strings.Contains(string(output), "Snirect Root CA")
}

func forceInstallCertPlatform(certPath string) error {
	logger.Info("Force installing certificate: %s", certPath)

	uninstallCertPlatform(certPath)
	return installCertPlatform(certPath)
}

func uninstallCertPlatform(certPath string) error {
	logger.Info("Attempting to uninstall certificate from Windows certificate store")

	cmd := exec.Command("certutil", "-user", "-delstore", "Root", "Snirect Root CA")
	output, err := cmd.CombinedOutput()

	if err != nil {
		outputStr := string(output)
		if strings.Contains(outputStr, "not found") || strings.Contains(outputStr, "No certificates") {
			logger.Info("Certificate was not found in Windows certificate store")
			return nil
		}
		return fmt.Errorf("failed to uninstall certificate: %v, output: %s", err, outputStr)
	}

	logger.Info("Certificate uninstalled successfully from Windows certificate store!")
	return nil
}

func checkCertStatusPlatform(certPath string) (bool, error) {
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return false, fmt.Errorf("failed to read certificate: %v", err)
	}

	installed := isCertInstalled(certData)
	return installed, nil
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

// HasTool checks if a system tool is available in PATH
func HasTool(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
