//go:build darwin

package sysproxy

import (
	"fmt"
	"github.com/xihale/snirect/internal/logger"
	"os"
	"os/exec"
	"strings"
)

func installCertPlatform(certPath string) (bool, error) {
	if isCertInstalled(certPath) {
		logger.System().Info("root certificate already installed in system trust store")
		return false, nil
	}

	logger.System().Info("installing certificate", "path", certPath)
	keychainPath := os.ExpandEnv("$HOME/Library/Keychains/login.keychain-db")

	cmd := exec.Command("security", "add-trusted-cert", "-d", "-r", "trustRoot", "-k", keychainPath, certPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	logger.System().Info("running certificate install command", "command", "security add-trusted-cert", "keychain", keychainPath, "cert_path", certPath)

	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("安装证书失败: %v，请手动安装证书: %s", err, certPath)
	}

	logger.System().Info("certificate installed")
	return true, nil
}

func isCertInstalled(certPath string) bool {
	fingerprint, err := GetCertFingerprint(certPath)
	if err != nil {
		return false
	}

	cmd := exec.Command("security", "find-certificate", "-a", "-c", "Snirect Root CA", "-p")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	installedCerts := strings.Split(string(output), "-----END CERTIFICATE-----")

	for _, installedCert := range installedCerts {
		if strings.TrimSpace(installedCert) == "" {
			continue
		}
		pemBlock := strings.TrimSpace(installedCert) + "\n-----END CERTIFICATE-----\n"
		installedFingerprint, err := GetCertFingerprintFromPEM([]byte(pemBlock))
		if err == nil && installedFingerprint == fingerprint {
			return true
		}
	}

	return false
}

func forceInstallCertPlatform(certPath string) (bool, error) {
	logger.System().Info("force installing certificate", "path", certPath)

	uninstallCertPlatform(certPath)
	return installCertPlatform(certPath)
}

func uninstallCertPlatform(certPath string) error {
	logger.System().Info("trying to uninstall certificate")

	keychainPath := os.ExpandEnv("$HOME/Library/Keychains/login.keychain-db")

	cmd := exec.Command("security", "delete-certificate", "-c", "Snirect Root CA", keychainPath)
	output, err := cmd.CombinedOutput()

	if err != nil {
		if strings.Contains(string(output), "SecKeychainSearchCopyNext") {
			logger.System().Info("certificate not found")
			return nil
		}
		return fmt.Errorf("卸载证书失败: %v, 输出: %s", err, string(output))
	}

	logger.System().Info("certificate uninstalled")
	return nil
}

func checkCertStatusPlatform(certPath string) (bool, error) {
	installed := isCertInstalled(certPath)
	return installed, nil
}

func setPACPlatform(pacURL string) {
	interfaces, err := getNetworkInterfaces()
	if err != nil {
		logger.System().Warn("failed to get network interfaces, cannot set proxy", "error", err)
		return
	}

	if len(interfaces) == 0 {
		logger.System().Warn("no active network interfaces found, cannot set proxy")
		return
	}

	for _, iface := range interfaces {
		logger.System().Info("setting PAC proxy for interface", "interface", iface)
		cmd := exec.Command("networksetup", "-setautoproxyurl", iface, pacURL)
		if output, err := cmd.CombinedOutput(); err != nil {
			logger.System().Debug("failed to set proxy for interface", "interface", iface, "error", err, "output", string(output))
		} else {
			logger.System().Info("proxy set for interface", "interface", iface)
		}
	}
}

func clearPACPlatform() {
	interfaces, err := getNetworkInterfaces()
	if err != nil {
		logger.System().Debug("failed to get network interfaces", "error", err)
		return
	}

	for _, iface := range interfaces {
		logger.System().Info("clearing PAC proxy for interface", "interface", iface)
		cmd := exec.Command("networksetup", "-setautoproxystate", iface, "off")
		if output, err := cmd.CombinedOutput(); err != nil {
			logger.System().Debug("failed to clear proxy for interface", "interface", iface, "error", err, "output", string(output))
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
