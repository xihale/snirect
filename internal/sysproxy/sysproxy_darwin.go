//go:build darwin

package sysproxy

import (
	"fmt"
	"os"
	"os/exec"
	"snirect/internal/logger"
	"strings"
)

func checkEnvPlatform(env map[string]string) {
	tools := []string{"security", "networksetup"}
	for _, tool := range tools {
		path, err := exec.LookPath(tool)
		if err == nil {
			env["Tool_"+tool] = path
		} else {
			env["Tool_"+tool] = "not found"
		}
	}

	interfaces, err := getNetworkInterfaces()
	if err == nil && len(interfaces) > 0 {
		env["NetworkInterface"] = interfaces[0]
	}
}

func installCertPlatform(certPath string) error {
	logger.Info("Attempting to install certificate: %s", certPath)

	keychainPath := os.ExpandEnv("$HOME/Library/Keychains/login.keychain-db")

	cmd := exec.Command("security", "add-trusted-cert", "-d", "-r", "trustRoot", "-k", keychainPath, certPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	logger.Info("Running: security add-trusted-cert -d -r trustRoot -k %s %s", keychainPath, certPath)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install certificate: %v (you may need to manually trust the certificate in Keychain Access)", err)
	}

	logger.Info("Certificate installed successfully!")
	logger.Info("Note: You may need to restart applications for changes to take effect.")
	return nil
}

func setPACPlatform(pacURL string) {
	interfaces, err := getNetworkInterfaces()
	if err != nil {
		logger.Warn("Failed to get network interfaces: %v. Cannot set proxy.", err)
		return
	}

	if len(interfaces) == 0 {
		logger.Warn("No active network interfaces found. Cannot set proxy.")
		return
	}

	for _, iface := range interfaces {
		logger.Info("Setting PAC proxy for interface: %s", iface)
		cmd := exec.Command("networksetup", "-setautoproxyurl", iface, pacURL)
		if output, err := cmd.CombinedOutput(); err != nil {
			logger.Debug("Failed to set proxy for %s: %v, output: %s", iface, err, string(output))
		} else {
			logger.Info("Proxy set successfully for %s", iface)
		}
	}
}

func clearPACPlatform() {
	interfaces, err := getNetworkInterfaces()
	if err != nil {
		logger.Debug("Failed to get network interfaces: %v", err)
		return
	}

	for _, iface := range interfaces {
		logger.Info("Clearing PAC proxy for interface: %s", iface)
		cmd := exec.Command("networksetup", "-setautoproxystate", iface, "off")
		if output, err := cmd.CombinedOutput(); err != nil {
			logger.Debug("Failed to clear proxy for %s: %v, output: %s", iface, err, string(output))
		}
	}
}

func getNetworkInterfaces() ([]string, error) {
	cmd := exec.Command("networksetup", "-listallnetworkservices")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	var interfaces []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "*") || strings.Contains(line, "An asterisk") {
			continue
		}
		interfaces = append(interfaces, line)
	}

	return interfaces, nil
}
