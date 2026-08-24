//go:build linux

package sysproxy

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/xihale/snirect/internal/logger"
)

func installCertPlatform(certPath string) (bool, error) {
	if isCertInstalled(certPath) {
		logger.System().Info("root certificate already installed in system trust store")
		return false, nil
	}

	logger.System().Info("installing certificate", "path", certPath)

	// Remove any previously installed Snirect CA from p11-kit trust source
	// to prevent duplicates (trust anchor --store creates .1, .2, ... suffixes)
	cleanSnirectPK11TrustAnchors()

	// Prefer the distro's standard trust-store layout when present:
	// Debian/Ubuntu keep local anchors in /usr/local/share/ca-certificates
	// and refresh via update-ca-certificates, which is the documented way
	// (sudo cp your.crt /usr/local/share/ca-certificates/ && sudo update-ca-certificates).
	if isDebianTrustStore() {
		logger.System().Info("detected Debian-style trust store, installing via update-ca-certificates")
		return installCertViaAnchors(certPath)
	}

	// Use trust tool (p11-kit)
	if path, err := exec.LookPath("trust"); err == nil {
		logger.System().Info("installing certificate with trust tool")
		cmd := exec.Command("sudo", path, "anchor", "--store", certPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		logger.System().Info("running certificate install command", "command", "sudo", "tool", path, "args", []string{"anchor", "--store", certPath})
		if err := cmd.Run(); err != nil {
			return false, fmt.Errorf("使用 trust 工具安装证书失败: %v\n请尝试手动运行: sudo trust anchor --store %s", err, certPath)
		}

		if _, err := exec.LookPath("update-ca-trust"); err == nil {
			_ = exec.Command("sudo", "update-ca-trust", "extract").Run()
		}

		if isCertInstalled(certPath) {
			return true, nil
		}
		return false, fmt.Errorf("使用 trust 工具安装后仍未检测到证书。请尝试手动安装。")
	}

	logger.System().Info("trust tool not found, trying legacy certificate install")
	return installCertViaAnchors(certPath)
}

// isDebianTrustStore reports whether this system uses the Debian/Ubuntu CA
// layout: local anchors dropped into /usr/local/share/ca-certificates and
// merged into the bundle by update-ca-certificates.
func isDebianTrustStore() bool {
	if _, err := os.Stat("/usr/local/share/ca-certificates/"); err != nil {
		return false
	}
	_, err := exec.LookPath("update-ca-certificates")
	return err == nil
}

// installCertViaAnchors copies the CA into the distro's anchor directory and
// refreshes the system bundle with the matching update command.
func installCertViaAnchors(certPath string) (bool, error) {
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
	logger.System().Info("writing certificate", "path", destPath)
	teeCmd := exec.Command("sudo", "tee", destPath)
	teeCmd.Stdin = strings.NewReader(string(data))
	teeCmd.Stdout = nil
	teeCmd.Stderr = os.Stderr
	if err := teeCmd.Run(); err != nil {
		return false, fmt.Errorf("写入证书文件失败: %v\n请手动将证书复制到 %s", err, destPath)
	}

	// 运行更新命令
	logger.System().Info("updating trust store", "command", updateCmd)
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
	// Parse our CA certificate.
	pemData, err := os.ReadFile(certPath)
	if err != nil {
		return false
	}
	block, _ := pem.Decode(pemData)
	if block == nil {
		return false
	}
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}

	// Match the CA against the system trust store *as it currently exists on
	// disk*. We deliberately do NOT use x509.SystemCertPool(): on Linux that
	// function is backed by a sync.Once that reads the bundle
	// (/etc/ssl/certs/ca-certificates.crt and friends) exactly once per process
	// and serves a stale snapshot on every later call. Because installCertPlatform
	// checks isCertInstalled both before and after running update-ca-certificates,
	// the pre-install check would warm the cache against a bundle that does not
	// yet contain our CA — making the post-install check report a false "not
	// installed" right after a successful install. Re-reading the bundle fresh
	// each call avoids that whole class of staleness bugs.
	pool := loadLinuxSystemRoots()
	if pool == nil {
		return false
	}
	_, err = caCert.Verify(x509.VerifyOptions{Roots: pool, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}})
	return err == nil
}

// loadLinuxSystemRoots reads the system root CA bundle directly from disk,
// mirroring the file/directory search order Go's crypto/x509 uses internally
// (see root_linux.go) but bypassing the process-wide sync.Once cache. We honor
// SSL_CERT_FILE / SSL_CERT_DIR the same way the stdlib does so the result stays
// consistent with everything else that trusts the system store.
func loadLinuxSystemRoots() *x509.CertPool {
	files := []string{
		"/etc/ssl/certs/ca-certificates.crt",                // Debian/Ubuntu/Gentoo etc.
		"/etc/pki/tls/certs/ca-bundle.crt",                  // Fedora/RHEL 6
		"/etc/ssl/ca-bundle.pem",                            // OpenSUSE
		"/etc/pki/tls/cacert.pem",                           // OpenELEC
		"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem", // CentOS/RHEL 7
		"/etc/ssl/cert.pem",                                 // Alpine Linux
	}
	if f := os.Getenv("SSL_CERT_FILE"); f != "" {
		files = []string{f}
	}
	dirs := []string{
		"/etc/ssl/certs",     // SLES10/SLES11
		"/etc/pki/tls/certs", // Fedora/RHEL
	}
	if d := os.Getenv("SSL_CERT_DIR"); d != "" {
		dirs = strings.Split(d, ":")
	}

	pool := x509.NewCertPool()
	for _, file := range files {
		if data, err := os.ReadFile(file); err == nil {
			pool.AppendCertsFromPEM(data)
			break // first existing bundle wins, matching the stdlib
		}
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if data, err := os.ReadFile(dir + "/" + e.Name()); err == nil {
				pool.AppendCertsFromPEM(data)
			}
		}
	}
	return pool
}

func forceInstallCertPlatform(certPath string) (bool, error) {
	logger.System().Info("force installing certificate", "path", certPath)

	_ = uninstallCertPlatform(certPath)
	return installCertPlatform(certPath)
}

func uninstallCertPlatform(certPath string) error {
	logger.System().Info("trying to uninstall certificate")

	// Clean p11-kit trust source entries
	cleanSnirectPK11TrustAnchors()

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
			logger.System().Info("removing certificate", "path", path)
			rmCmd := exec.Command("sudo", "rm", path)
			rmCmd.Stdout = os.Stdout
			rmCmd.Stderr = os.Stderr
			if err := rmCmd.Run(); err != nil {
				logger.System().Warn("failed to remove certificate", "path", path, "error", err)
			} else {
				removed = true
			}
		}
	}

	if path, err := exec.LookPath("update-ca-certificates"); err == nil {
		logger.System().Info("updating CA certificates")
		upCmd := exec.Command("sudo", path)
		upCmd.Stdout = os.Stdout
		upCmd.Stderr = os.Stderr
		if err := upCmd.Run(); err != nil {
			return fmt.Errorf("failed to update trust store: %v", err)
		}
		removed = true
	} else if path, err := exec.LookPath("update-ca-trust"); err == nil {
		logger.System().Info("updating CA trust store")
		upCmd := exec.Command("sudo", path)
		upCmd.Stdout = os.Stdout
		upCmd.Stderr = os.Stderr
		if err := upCmd.Run(); err != nil {
			return fmt.Errorf("failed to update trust store: %v", err)
		}
		removed = true
	}

	if removed {
		logger.System().Info("certificate uninstalled")
	} else {
		logger.System().Info("certificate not found in system trust store")
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
		if HasTool("gsettings") {
			logger.System().Info("detected GNOME-like environment, setting proxy via gsettings")
			setGnomeProxy(pacURL)
		} else {
			logger.System().Warn("detected GNOME-like environment but gsettings not found, cannot set proxy")
		}
	case "kde":
		if HasTool("kwriteconfig5") {
			logger.System().Info("detected KDE, setting proxy via kwriteconfig5")
			setKDEProxy(pacURL)
		} else {
			logger.System().Warn("detected KDE but kwriteconfig5 not found, cannot set proxy")
		}
	default:
		logger.System().Warn("auto-proxy setting not supported for desktop environment", "desktop_environment", de)
	}
}

func clearPACPlatform() {
	de := getDesktopEnvironment()
	switch de {
	case "gnome":
		if HasTool("gsettings") {
			clearGnomeProxy()
		}
	case "kde":
		if HasTool("kwriteconfig5") {
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

	if HasTool("dbus-send") {
		runCommand("dbus-send", "--type=signal", "/KIO/Scheduler", "org.kde.KIO.Scheduler.reparseSlaveConfiguration", "string:''")
	}
}

func clearKDEProxy() {
	runCommand("kwriteconfig5", "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "ProxyType", "0")
	runCommand("kwriteconfig5", "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "Proxy Config Script", "")

	if HasTool("dbus-send") {
		runCommand("dbus-send", "--type=signal", "/KIO/Scheduler", "org.kde.KIO.Scheduler.reparseSlaveConfiguration", "string:''")
	}
}

func runCommand(name string, args ...string) {
	cmd := exec.Command(name, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.System().Debug("command failed", "command", name, "args", args, "error", err, "output", string(output))
	} else {
		logger.System().Debug("command executed", "command", name, "args", args)
	}
}

func cleanSnirectPK11TrustAnchors() {
	// p11-kit trust anchor --store creates files in /etc/ca-certificates/trust-source/
	// with .p11-kit extension. Repeated installs get .1, .2, ... suffixes.
	patterns := []string{
		"/etc/ca-certificates/trust-source/Snirect_Root_CA*.p11-kit",
	}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, f := range matches {
			logger.System().Info("removing old p11-kit trust anchor", "path", f)
			rmCmd := exec.Command("sudo", "rm", "-f", f)
			rmCmd.Stdout = os.Stdout
			rmCmd.Stderr = os.Stderr
			if err := rmCmd.Run(); err != nil {
				logger.System().Warn("failed to remove p11-kit anchor", "path", f, "error", err)
			}
		}
	}
}
