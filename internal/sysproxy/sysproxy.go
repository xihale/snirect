package sysproxy

import (
	"snirect/internal/logger"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// CheckEnv returns a map of detected environment details.
func CheckEnv() map[string]string {
	env := make(map[string]string)
	env["OS"] = runtime.GOOS
	env["DesktopEnvironment"] = getDesktopEnvironment()
	
	// Check Cert Tools
	tools := []string{"trust", "update-ca-certificates", "update-ca-trust"}
	for _, tool := range tools {
		path, err := exec.LookPath(tool)
		if err == nil {
			env["CertTool_"+tool] = path
		} else {
			env["CertTool_"+tool] = "not found"
		}
	}
	
	// Check Proxy Tools
	proxyTools := []string{"gsettings", "kwriteconfig5", "dbus-send"}
	for _, tool := range proxyTools {
		path, err := exec.LookPath(tool)
		if err == nil {
			env["ProxyTool_"+tool] = path
		} else {
			env["ProxyTool_"+tool] = "not found"
		}
	}

	return env
}

// InstallCert attempts to install the CA certificate to the system trust store.
func InstallCert(certPath string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("certificate installation is currently only supported on Linux")
	}

	logger.Info("Attempting to install certificate: %s", certPath)

	var destPath string
	var updateCmd string
	var updateArgs []string

	// Prioritize 'trust' (p11-kit) as it is the modern standard (Arch, Fedora, etc.)
	if path, err := exec.LookPath("trust"); err == nil {
		// sudo trust anchor --store cert.crt
		cmd := exec.Command("sudo", path, "anchor", "--store", certPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		logger.Info("Running: sudo %s anchor --store %s", path, certPath)
		return cmd.Run()
	} else if path, err := exec.LookPath("update-ca-certificates"); err == nil {
		// Debian/Ubuntu
		destPath = "/usr/local/share/ca-certificates/snirect-root.crt"
		updateCmd = path
	} else if path, err := exec.LookPath("update-ca-trust"); err == nil {
		// Legacy RHEL/Fedora
		destPath = "/etc/pki/ca-trust/source/anchors/snirect-root.crt"
		updateCmd = path
	} else {
		return fmt.Errorf("could not detect certificate management tool (trust, update-ca-certificates, or update-ca-trust)")
	}

	// For legacy methods that require copying
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

// SetPAC sets the system proxy auto-config URL.
func SetPAC(pacURL string) {
	if runtime.GOOS == "windows" {
		logger.Info("Setting system proxy (Windows implementation pending)...")
	} else if runtime.GOOS == "linux" {
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
}

// ClearPAC removes the system proxy auto-config URL.
func ClearPAC() {
	if runtime.GOOS == "windows" {
		logger.Info("Clearing system proxy (Windows implementation pending)...")
	} else if runtime.GOOS == "linux" {
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

func hasTool(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
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