//go:build linux

package sysproxy

import (
	"fmt"
	"os"
	"os/exec"
	"snirect/internal/logger"
	"strings"
)

func checkEnvPlatform(env map[string]string) {
	env["DesktopEnvironment"] = getDesktopEnvironment()

	tools := []string{"trust", "update-ca-certificates", "update-ca-trust"}
	for _, tool := range tools {
		path, err := exec.LookPath(tool)
		if err == nil {
			env["CertTool_"+tool] = path
		} else {
			env["CertTool_"+tool] = "not found"
		}
	}

	proxyTools := []string{"gsettings", "kwriteconfig5", "dbus-send"}
	for _, tool := range proxyTools {
		path, err := exec.LookPath(tool)
		if err == nil {
			env["ProxyTool_"+tool] = path
		} else {
			env["ProxyTool_"+tool] = "not found"
		}
	}
}

func installCertPlatform(certPath string) (bool, error) {
	if isCertInstalled(certPath) {
		logger.Info("根证书已安装在系统信任库中")
		return false, nil
	}

	logger.Info("正在安装证书: %s", certPath)

	// 优先使用 trust 工具 (p11-kit)
	if path, err := exec.LookPath("trust"); err == nil {
		logger.Info("正在使用 trust 工具安装证书...")
		cmd := exec.Command("sudo", path, "anchor", certPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return false, fmt.Errorf("使用 trust 工具安装证书失败 (错误 13 可能表示权限问题或不支持的格式): %v\n请尝试手动运行: sudo trust anchor %s", err, certPath)
		}

		// 某些系统可能需要显式执行 extract
		if _, err := exec.LookPath("update-ca-trust"); err == nil {
			exec.Command("sudo", "update-ca-trust", "extract").Run()
		}

		if isCertInstalled(certPath) {
			return true, nil
		}
		return false, fmt.Errorf("使用 trust 工具安装后仍未检测到证书。请尝试手动安装。")
	}

	// 如果没有 trust 工具，回退到传统的路径检测
	logger.Info("未找到 trust 工具，尝试传统安装方式...")
	var destPath string
	var updateCmd string
	var updateArgs []string

	if _, err := os.Stat("/etc/pki/ca-trust/source/anchors/"); err == nil {
		destPath = "/etc/pki/ca-trust/source/anchors/snirect-root.pem"
		updateCmd = "update-ca-trust"
		updateArgs = []string{"extract"}
	} else if _, err := os.Stat("/usr/local/share/ca-certificates/"); err == nil {
		destPath = "/usr/local/share/ca-certificates/snirect-root.crt"
		updateCmd = "update-ca-certificates"
	} else if _, err := os.Stat("/etc/ca-certificates/trust-source/anchors/"); err == nil {
		destPath = "/etc/ca-certificates/trust-source/anchors/snirect-root.crt"
		updateCmd = "update-ca-certificates" // Arch uses update-ca-trust but update-ca-certificates might be present
		if _, err := exec.LookPath("update-ca-trust"); err == nil {
			updateCmd = "update-ca-trust"
			updateArgs = []string{"extract"}
		}
	} else if _, err := os.Stat("/usr/share/pki/trust/anchors/"); err == nil {
		destPath = "/usr/share/pki/trust/anchors/snirect-root.pem"
		updateCmd = "update-ca-certificates"
	} else {
		return false, fmt.Errorf("不支持的 Linux 发行版：无法检测到标准的 CA 安装路径，且未找到 trust 工具。\n请将证书手动添加到系统信任库中。")
	}

	// 读取证书数据
	data, err := os.ReadFile(certPath)
	if err != nil {
		return false, err
	}

	// 写入证书文件
	logger.Info("正在写入证书到 %s...", destPath)
	teeCmd := exec.Command("sudo", "tee", destPath)
	teeCmd.Stdin = strings.NewReader(string(data))
	teeCmd.Stdout = nil
	teeCmd.Stderr = os.Stderr
	if err := teeCmd.Run(); err != nil {
		return false, fmt.Errorf("写入证书文件失败: %v\n请手动将证书复制到 %s", err, destPath)
	}

	// 运行更新命令
	logger.Info("正在更新信任库 (%s)...", updateCmd)
	upCmd := exec.Command("sudo", append([]string{updateCmd}, updateArgs...)...)
	upCmd.Stdout = os.Stdout
	upCmd.Stderr = os.Stderr
	if err := upCmd.Run(); err != nil {
		return false, fmt.Errorf("更新信任库失败: %v\n请尝试手动运行: sudo %s %s", err, updateCmd, strings.Join(updateArgs, " "))
	}

	// 验证安装
	if !isCertInstalled(certPath) {
		return false, fmt.Errorf("安装似乎完成了，但在系统信任库中仍未检测到证书。\n请重启应用或尝试手动安装。")
	}

	return true, nil
}

func isCertInstalled(certPath string) bool {
	// Bypass Go's x509.SystemCertPool caching by running a separate process
	// The separate process will load a fresh SystemCertPool
	cmd := exec.Command(os.Args[0], "verify-cert", certPath)
	if err := cmd.Run(); err == nil {
		return true
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

	certPaths := []string{
		"/usr/local/share/ca-certificates/snirect-root.crt",
		"/etc/pki/ca-trust/source/anchors/snirect-root.crt",
		"/etc/pki/ca-trust/source/anchors/snirect-root.pem",
		"/etc/ca-certificates/trust-source/anchors/snirect-root.crt",
		"/usr/share/pki/trust/anchors/snirect-root.pem",
	}

	removed := false

	for _, path := range certPaths {
		if _, err := os.Stat(path); err == nil {
			logger.Info("正在从 %s 移除证书...", path)
			rmCmd := exec.Command("sudo", "rm", path)
			rmCmd.Stdout = os.Stdout
			rmCmd.Stderr = os.Stderr
			if err := rmCmd.Run(); err != nil {
				logger.Warn("从 %s 移除证书失败: %v", path, err)
			} else {
				removed = true
			}
		}
	}

	if path, err := exec.LookPath("trust"); err == nil {
		logger.Info("正在使用 trust 工具从信任库中移除证书...")
		cmd := exec.Command("sudo", path, "anchor", "--remove", certPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logger.Warn("使用 trust 移除证书失败: %v", err)
		} else {
			removed = true
		}
	} else if path, err := exec.LookPath("update-ca-certificates"); err == nil {
		logger.Info("正在更新 CA 证书...")
		upCmd := exec.Command("sudo", path)
		upCmd.Stdout = os.Stdout
		upCmd.Stderr = os.Stderr
		if err := upCmd.Run(); err != nil {
			return fmt.Errorf("failed to update trust store: %v", err)
		}
		removed = true
	} else if path, err := exec.LookPath("update-ca-trust"); err == nil {
		logger.Info("正在更新 CA 信任库...")
		upCmd := exec.Command("sudo", path)
		upCmd.Stdout = os.Stdout
		upCmd.Stderr = os.Stderr
		if err := upCmd.Run(); err != nil {
			return fmt.Errorf("failed to update trust store: %v", err)
		}
		removed = true
	}

	if removed {
		logger.Info("证书卸载成功！")
	} else {
		logger.Info("在系统信任库中未找到证书")
	}

	return nil
}

func checkCertStatusPlatform(certPath string) (bool, error) {
	installed := isCertInstalled(certPath)
	return installed, nil
}

func setPACPlatform(pacURL string) {
	de := getDesktopEnvironment()
	switch de {
	case "gnome":
		if hasTool("gsettings") {
			logger.Info("Detected GNOME-like environment, setting proxy via gsettings...")
			setGnomeProxy(pacURL)
		} else {
			logger.Warn("Detected GNOME-like environment but 'gsettings' not found. Cannot set proxy.")
		}
	case "kde":
		if hasTool("kwriteconfig5") {
			logger.Info("Detected KDE, setting proxy via kwriteconfig5...")
			setKDEProxy(pacURL)
		} else {
			logger.Warn("Detected KDE but 'kwriteconfig5' not found. Cannot set proxy.")
		}
	default:
		logger.Warn("Auto-proxy setting not supported for desktop environment: %s. Please set manually.", de)
	}
}

func clearPACPlatform() {
	de := getDesktopEnvironment()
	switch de {
	case "gnome":
		if hasTool("gsettings") {
			clearGnomeProxy()
		}
	case "kde":
		if hasTool("kwriteconfig5") {
			clearKDEProxy()
		}
	}
}

func getDesktopEnvironment() string {
	xdg := os.Getenv("XDG_CURRENT_DESKTOP")
	if xdg == "" {
		xdg = os.Getenv("DESKTOP_SESSION")
	}
	xdg = strings.ToLower(xdg)

	if strings.Contains(xdg, "gnome") || strings.Contains(xdg, "unity") || strings.Contains(xdg, "deepin") || strings.Contains(xdg, "pantheon") {
		return "gnome"
	}
	if strings.Contains(xdg, "kde") || strings.Contains(xdg, "plasma") {
		return "kde"
	}
	return xdg
}

// HasTool checks if a system tool is available in PATH
func HasTool(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// Deprecated: use HasTool instead
func hasTool(name string) bool {
	return HasTool(name)
}

func setGnomeProxy(pacURL string) {
	runCommand("gsettings", "set", "org.gnome.system.proxy", "mode", "auto")
	runCommand("gsettings", "set", "org.gnome.system.proxy", "autoconfig-url", pacURL)
}

func clearGnomeProxy() {
	runCommand("gsettings", "set", "org.gnome.system.proxy", "mode", "none")
	runCommand("gsettings", "set", "org.gnome.system.proxy", "autoconfig-url", "")
}

func setKDEProxy(pacURL string) {
	runCommand("kwriteconfig5", "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "ProxyType", "2")
	runCommand("kwriteconfig5", "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "Proxy Config Script", pacURL)

	if hasTool("dbus-send") {
		runCommand("dbus-send", "--type=signal", "/KIO/Scheduler", "org.kde.KIO.Scheduler.reparseSlaveConfiguration", "string:''")
	}
}

func clearKDEProxy() {
	runCommand("kwriteconfig5", "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "ProxyType", "0")
	runCommand("kwriteconfig5", "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "Proxy Config Script", "")

	if hasTool("dbus-send") {
		runCommand("dbus-send", "--type=signal", "/KIO/Scheduler", "org.kde.KIO.Scheduler.reparseSlaveConfiguration", "string:''")
	}
}

func runCommand(name string, args ...string) {
	cmd := exec.Command(name, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.Debug("Failed to run %s %s: %v, output: %s", name, strings.Join(args, " "), err, string(output))
	} else {
		logger.Debug("Executed: %s %s", name, strings.Join(args, " "))
	}
}
