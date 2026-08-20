//go:build windows

package sysproxy

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/xihale/snirect/logger"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	modwininet            = windows.NewLazySystemDLL("wininet.dll")
	procInternetSetOption = modwininet.NewProc("InternetSetOptionW")
)

const (
	INTERNET_OPTION_SETTINGS_CHANGED = 39
	INTERNET_OPTION_REFRESH          = 37
)

func installCertPlatform(certPath string) (bool, error) {
	if isCertInstalled(certPath) {
		logger.System().Info("root certificate already installed in system trust store")
		return false, nil
	}

	logger.System().Info("installing certificate", "path", certPath)
	cmd := exec.Command("certutil", "-addstore", "-user", "Root", certPath)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return false, fmt.Errorf("安装证书失败: %v, 输出: %s", err, string(output))
	}

	logger.System().Info("certificate installed")
	return true, nil
}

func isCertInstalled(certPath string) bool {
	cmd := exec.Command("certutil", "-user", "-verifystore", "Root", "Snirect Root CA")
	if err := cmd.Run(); err != nil {
		return false
	}

	sha1, err := GetCertFingerprintSHA1(certPath)
	if err != nil {
		return false
	}

	cmd = exec.Command("certutil", "-user", "-verifystore", "Root", sha1)
	err = cmd.Run()
	return err == nil
}

func forceInstallCertPlatform(certPath string) (bool, error) {
	logger.System().Info("force installing certificate", "path", certPath)

	uninstallCertPlatform(certPath)
	return installCertPlatform(certPath)
}

func uninstallCertPlatform(certPath string) error {
	logger.System().Info("trying to uninstall certificate")

	cmd := exec.Command("certutil", "-user", "-delstore", "Root", "Snirect Root CA")
	output, err := cmd.CombinedOutput()

	if err != nil {
		outputStr := string(output)
		if strings.Contains(outputStr, "not found") || strings.Contains(outputStr, "No certificates") {
			logger.System().Info("certificate not found")
			return nil
		}
		return fmt.Errorf("卸载证书失败: %v, 输出: %s", err, outputStr)
	}

	logger.System().Info("certificate uninstalled")
	return nil
}

func checkCertStatusPlatform(certPath string) (bool, error) {
	installed := isCertInstalled(certPath)
	return installed, nil
}

func setPACPlatform(pacURL string) {
	logger.System().Info("setting system proxy", "pac_url", pacURL)

	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		registry.SET_VALUE)
	if err != nil {
		logger.System().Warn("failed to open registry", "error", err)
		return
	}
	defer key.Close()

	if err := key.SetStringValue("AutoConfigURL", pacURL); err != nil {
		logger.System().Warn("failed to set AutoConfigURL", "error", err)
		return
	}

	notifyProxyChange()

	logger.System().Info("system proxy set")
}

func clearPACPlatform() {
	logger.System().Info("clearing system proxy")

	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		registry.SET_VALUE)
	if err != nil {
		logger.System().Debug("failed to open registry", "error", err)
		return
	}
	defer key.Close()

	if err := key.DeleteValue("AutoConfigURL"); err != nil {
		logger.System().Debug("failed to delete AutoConfigURL", "error", err)
	}

	notifyProxyChange()

	logger.System().Info("system proxy cleared")
}

func notifyProxyChange() {
	for i := 0; i < 3; i++ {
		procInternetSetOption.Call(0, uintptr(INTERNET_OPTION_SETTINGS_CHANGED), 0, 0)
		procInternetSetOption.Call(0, uintptr(INTERNET_OPTION_REFRESH), 0, 0)
		if i < 2 {
			windows.SleepEx(100, false)
		}
	}
}
