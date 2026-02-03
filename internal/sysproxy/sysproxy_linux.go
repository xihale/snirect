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

	var destPath string
	var updateCmd string
	var updateArgs []string

	if path, err := exec.LookPath("trust"); err == nil {
		cmd := exec.Command("sudo", path, "anchor", "--store", certPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		logger.Info("Running: sudo %s anchor --store %s", path, certPath)
		return cmd.Run()
	} else if path, err := exec.LookPath("update-ca-certificates"); err == nil {
		destPath = "/usr/local/share/ca-certificates/snirect-root.crt"
		updateCmd = path
	} else if path, err := exec.LookPath("update-ca-trust"); err == nil {
		destPath = "/etc/pki/ca-trust/source/anchors/snirect-root.crt"
		updateCmd = path
	} else {
		return fmt.Errorf("could not detect certificate management tool (trust, update-ca-certificates, or update-ca-trust)")
	}

	logger.Info("Copying certificate to %s...", destPath)
	cpCmd := exec.Command("sudo", "cp", certPath, destPath)
	cpCmd.Stdout = os.Stdout
	cpCmd.Stderr = os.Stderr
	if err := cpCmd.Run(); err != nil {
		return fmt.Errorf("failed to copy certificate: %v", err)
	}

	logger.Info("Updating trust store using %s...", updateCmd)
	cmdArgs := append([]string{updateCmd}, updateArgs...)
	upCmd := exec.Command("sudo", cmdArgs...)
	upCmd.Stdout = os.Stdout
	upCmd.Stderr = os.Stderr
	if err := upCmd.Run(); err != nil {
		return fmt.Errorf("failed to update trust store: %v", err)
	}

	logger.Info("Certificate installed successfully!")
	return nil
}

func isCertInstalled(certData []byte) bool {
	certPaths := []string{
		"/usr/local/share/ca-certificates/snirect-root.crt",
		"/etc/pki/ca-trust/source/anchors/snirect-root.crt",
	}

	for _, path := range certPaths {
		if data, err := os.ReadFile(path); err == nil {
			if string(data) == string(certData) {
				return true
			}
		}
	}

	if path, err := exec.LookPath("trust"); err == nil {
		cmd := exec.Command(path, "list")
		output, err := cmd.Output()
		if err == nil && strings.Contains(string(output), "Snirect Root CA") {
			return true
		}
	}

	return false
}

func forceInstallCertPlatform(certPath string) error {
	logger.Info("Force installing certificate: %s", certPath)

	uninstallCertPlatform(certPath)
	return installCertPlatform(certPath)
}

func uninstallCertPlatform(certPath string) error {
	logger.Info("Attempting to uninstall certificate")

	certPaths := []string{
		"/usr/local/share/ca-certificates/snirect-root.crt",
		"/etc/pki/ca-trust/source/anchors/snirect-root.crt",
	}

	removed := false

	for _, path := range certPaths {
		if _, err := os.Stat(path); err == nil {
			logger.Info("Removing certificate from %s...", path)
			rmCmd := exec.Command("sudo", "rm", path)
			rmCmd.Stdout = os.Stdout
			rmCmd.Stderr = os.Stderr
			if err := rmCmd.Run(); err != nil {
				logger.Warn("Failed to remove certificate from %s: %v", path, err)
			} else {
				removed = true
			}
		}
	}

	if path, err := exec.LookPath("trust"); err == nil {
		logger.Info("Removing certificate from trust store using trust tool...")
		cmd := exec.Command("sudo", path, "anchor", "--remove", certPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logger.Warn("Failed to remove certificate using trust: %v", err)
		} else {
			removed = true
		}
	} else if path, err := exec.LookPath("update-ca-certificates"); err == nil {
		logger.Info("Updating CA certificates...")
		upCmd := exec.Command("sudo", path)
		upCmd.Stdout = os.Stdout
		upCmd.Stderr = os.Stderr
		if err := upCmd.Run(); err != nil {
			return fmt.Errorf("failed to update trust store: %v", err)
		}
		removed = true
	} else if path, err := exec.LookPath("update-ca-trust"); err == nil {
		logger.Info("Updating CA trust...")
		upCmd := exec.Command("sudo", path)
		upCmd.Stdout = os.Stdout
		upCmd.Stderr = os.Stderr
		if err := upCmd.Run(); err != nil {
			return fmt.Errorf("failed to update trust store: %v", err)
		}
		removed = true
	}

	if removed {
		logger.Info("Certificate uninstalled successfully!")
	} else {
		logger.Info("Certificate was not found in system trust store")
	}

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
