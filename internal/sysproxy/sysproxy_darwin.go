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

func installCertPlatform(certPath string) (bool, error) {
	if isCertInstalled(certPath) {
		logger.Info("根证书已安装在系统信任库中")
		return false, nil
	}

	logger.Info("正在安装证书: %s", certPath)
	keychainPath := os.ExpandEnv("$HOME/Library/Keychains/login.keychain-db")

	cmd := exec.Command("security", "add-trusted-cert", "-d", "-r", "trustRoot", "-k", keychainPath, certPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	logger.Info("执行命令: security add-trusted-cert -d -r trustRoot -k %s %s", keychainPath, certPath)

	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("安装证书失败: %v，请手动安装证书: %s", err, certPath)
	}

	logger.Info("证书安装成功。")
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
	logger.Info("正在强制安装证书: %s", certPath)

	uninstallCertPlatform(certPath)
	return installCertPlatform(certPath)
}

func uninstallCertPlatform(certPath string) error {
	logger.Info("正在尝试卸载证书")

	keychainPath := os.ExpandEnv("$HOME/Library/Keychains/login.keychain-db")

	cmd := exec.Command("security", "delete-certificate", "-c", "Snirect Root CA", keychainPath)
	output, err := cmd.CombinedOutput()

	if err != nil {
		if strings.Contains(string(output), "SecKeychainSearchCopyNext") {
			logger.Info("未找到证书")
			return nil
		}
		return fmt.Errorf("卸载证书失败: %v, 输出: %s", err, string(output))
	}

	logger.Info("证书卸载成功。")
	return nil
}

func checkCertStatusPlatform(certPath string) (bool, error) {
	installed := isCertInstalled(certPath)
	return installed, nil
}

func setPACPlatform(pacURL string) {
	interfaces, err := getNetworkInterfaces()
	if err != nil {
		logger.Warn("获取网络接口失败: %v。无法设置代理。", err)
		return
	}

	if len(interfaces) == 0 {
		logger.Warn("未找到活跃的网络接口。无法设置代理。")
		return
	}

	for _, iface := range interfaces {
		logger.Info("正在为接口设置 PAC 代理: %s", iface)
		cmd := exec.Command("networksetup", "-setautoproxyurl", iface, pacURL)
		if output, err := cmd.CombinedOutput(); err != nil {
			logger.Debug("设置 %s 代理失败: %v, 输出: %s", iface, err, string(output))
		} else {
			logger.Info("成功为 %s 设置代理", iface)
		}
	}
}

func clearPACPlatform() {
	interfaces, err := getNetworkInterfaces()
	if err != nil {
		logger.Debug("获取网络接口失败: %v", err)
		return
	}

	for _, iface := range interfaces {
		logger.Info("正在清除接口的 PAC 代理: %s", iface)
		cmd := exec.Command("networksetup", "-setautoproxystate", iface, "off")
		if output, err := cmd.CombinedOutput(); err != nil {
			logger.Debug("清除 %s 代理失败: %v, 输出: %s", iface, err, string(output))
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

func isLaunchedBySystemOrGUIPlatform() bool {
	return false
}

func hideConsolePlatform() {}

func disableColorPlatform() {}
