//go:build windows

package sysproxy

import (
	"fmt"
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

func installCertPlatform(certPath string) (bool, error) {
	if isCertInstalled(certPath) {
		logger.Info("根证书已安装在系统信任库中")
		return false, nil
	}

	logger.Info("正在安装证书: %s", certPath)
	cmd := exec.Command("certutil", "-addstore", "-user", "Root", certPath)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return false, fmt.Errorf("安装证书失败: %v, 输出: %s", err, string(output))
	}

	logger.Info("证书安装成功！")
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
	logger.Info("正在强制安装证书: %s", certPath)

	uninstallCertPlatform(certPath)
	return installCertPlatform(certPath)
}

func uninstallCertPlatform(certPath string) error {
	logger.Info("正在尝试卸载证书")

	cmd := exec.Command("certutil", "-user", "-delstore", "Root", "Snirect Root CA")
	output, err := cmd.CombinedOutput()

	if err != nil {
		outputStr := string(output)
		if strings.Contains(outputStr, "not found") || strings.Contains(outputStr, "No certificates") {
			logger.Info("未找到证书")
			return nil
		}
		return fmt.Errorf("卸载证书失败: %v, 输出: %s", err, outputStr)
	}

	logger.Info("证书卸载成功！")
	return nil
}

func checkCertStatusPlatform(certPath string) (bool, error) {
	installed := isCertInstalled(certPath)
	return installed, nil
}

func setPACPlatform(pacURL string) {
	logger.Info("正在将系统代理设置为: %s", pacURL)

	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		registry.SET_VALUE)
	if err != nil {
		logger.Warn("打开注册表失败: %v", err)
		return
	}
	defer key.Close()

	if err := key.SetStringValue("AutoConfigURL", pacURL); err != nil {
		logger.Warn("设置 AutoConfigURL 失败: %v", err)
		return
	}

	logger.Info("系统代理设置成功。部分应用可能需要重启才能生效。")
}

func clearPACPlatform() {
	logger.Info("正在清除系统代理设置...")

	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		registry.SET_VALUE)
	if err != nil {
		logger.Debug("打开注册表失败: %v", err)
		return
	}
	defer key.Close()

	if err := key.DeleteValue("AutoConfigURL"); err != nil {
		logger.Debug("删除 AutoConfigURL 失败: %v", err)
	}

	logger.Info("系统代理已清除。")
}

// HasTool checks if a system tool is available in PATH
func HasTool(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
