package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"snirect/internal/config"
	"snirect/internal/logger"
	"snirect/internal/sysproxy"
	"strings"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check Snirect status (service, certificate, proxy)",
	Long: `Display comprehensive status information about Snirect:
  - Whether the proxy service is running
  - Root CA certificate installation status
  - System proxy configuration
  - Configuration file locations`,
	Run: func(cmd *cobra.Command, args []string) {
		printStatus()
	},
}

func printStatus() {
	cyan := "\033[36m"
	green := "\033[32m"
	red := "\033[31m"
	yellow := "\033[33m"
	reset := "\033[0m"
	bold := "\033[1m"

	fmt.Printf("\n%s%sSnirect Status%s\n", bold, cyan, reset)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// 1. Check configuration
	appDir, err := config.GetAppDataDir()
	if err != nil {
		logger.Warn("Failed to get app directory: %v", err)
		appDir = "unknown"
	}

	configPath := filepath.Join(appDir, "config.toml")
	cfg, cfgErr := config.LoadConfig(configPath)

	fmt.Printf("%sConfiguration:%s\n", bold, reset)
	fmt.Printf("  Config directory: %s%s%s\n", cyan, appDir, reset)
	if cfgErr != nil {
		fmt.Printf("  Config status: %s%s%s (using defaults)\n", yellow, "⚠ Not loaded", reset)
	} else {
		fmt.Printf("  Config status: %s%s%s\n", green, "✓ Loaded", reset)
		fmt.Printf("  Server port: %s%d%s\n", cyan, cfg.Server.Port, reset)
	}
	fmt.Println()

	// 2. Check certificate
	certDir := filepath.Join(appDir, "certs")
	caCertPath := filepath.Join(certDir, "root.crt")

	fmt.Printf("%sCertificate:%s\n", bold, reset)
	if _, err := os.Stat(caCertPath); err == nil {
		fmt.Printf("  CA file: %s%s%s\n", green, "✓ Exists", reset)
		fmt.Printf("  Path: %s%s%s\n", cyan, caCertPath, reset)

		// Check if installed in system
		installed, err := sysproxy.CheckCertStatus(caCertPath)
		if err != nil {
			fmt.Printf("  System trust: %s%s%s (%v)\n", yellow, "⚠ Unknown", reset, err)
		} else if installed {
			fmt.Printf("  System trust: %s%s%s\n", green, "✓ Installed", reset)
		} else {
			fmt.Printf("  System trust: %s%s%s\n", red, "✗ Not installed", reset)
			fmt.Printf("    Run: %ssnirect install-cert%s\n", yellow, reset)
		}
	} else {
		fmt.Printf("  CA file: %s%s%s\n", red, "✗ Not found", reset)
		fmt.Printf("    Run: %ssnirect%s to generate\n", yellow, reset)
	}
	fmt.Println()

	// 3. Check proxy
	fmt.Printf("%sSystem Proxy:%s\n", bold, reset)
	if isProxySet() {
		fmt.Printf("  Status: %s%s%s\n", green, "✓ Enabled", reset)
		if cfg != nil {
			pacURL := fmt.Sprintf("http://127.0.0.1:%d/pac/", cfg.Server.Port)
			fmt.Printf("  PAC URL: %s%s%s\n", cyan, pacURL, reset)
		}
	} else {
		fmt.Printf("  Status: %s%s%s\n", red, "✗ Not set", reset)
		fmt.Printf("    Run: %ssnirect set-proxy%s\n", yellow, reset)
	}
	fmt.Println()

	// 4. Check service
	fmt.Printf("%sService:%s\n", bold, reset)
	serviceStatus := checkServiceStatus()
	fmt.Printf("  Status: %s\n", serviceStatus)
	fmt.Println()

	// 5. Quick tips
	fmt.Printf("%s%sQuick Commands:%s\n", bold, cyan, reset)
	fmt.Printf("  Start proxy:    %ssnirect%s\n", yellow, reset)
	fmt.Printf("  Install CA:       %ssnirect install-cert%s\n", yellow, reset)
	fmt.Printf("  Set proxy:        %ssnirect set-proxy%s\n", yellow, reset)
	fmt.Printf("  View logs:        %sjournalctl --user -u snirect -f%s (Linux)\n", yellow, reset)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
}

func isProxySet() bool {
	switch runtime.GOOS {
	case "linux":
		// Check GNOME
		if sysproxy.HasTool("gsettings") {
			cmd := exec.Command("gsettings", "get", "org.gnome.system.proxy", "mode")
			output, err := cmd.Output()
			if err == nil && strings.Contains(string(output), "auto") {
				return true
			}
		}
	case "darwin":
		if sysproxy.HasTool("networksetup") {
			cmd := exec.Command("networksetup", "-listallnetworkservices")
			services, err := cmd.Output()
			if err == nil {
				lines := strings.Split(string(services), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" || strings.HasPrefix(line, "*") {
						continue
					}
					cmd := exec.Command("networksetup", "-getautoproxyurl", line)
					output, err := cmd.Output()
					if err == nil && strings.Contains(string(output), "http://127.0.0.1") {
						return true
					}
				}
			}
		}
	case "windows":
		// Check Windows registry
		cmd := exec.Command("reg", "query", "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Internet", "/v", "AutoConfigURL")
		output, err := cmd.Output()
		if err == nil && strings.Contains(string(output), "http://") {
			return true
		}
	}
	return false
}

func checkServiceStatus() string {
	green := "\033[32m"
	red := "\033[31m"
	yellow := "\033[33m"
	reset := "\033[0m"

	switch runtime.GOOS {
	case "linux":
		if !sysproxy.HasTool("systemctl") {
			return yellow + "⚠ Cannot check (systemctl not found)" + reset
		}
		cmd := exec.Command("systemctl", "--user", "is-active", "snirect")
		output, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(output)) == "active" {
			return green + "✓ Running" + reset
		}
		cmd = exec.Command("systemctl", "--user", "is-enabled", "snirect")
		output, _ = cmd.Output()
		if strings.TrimSpace(string(output)) == "enabled" {
			return yellow + "⚡ Enabled (not running)" + reset
		}
		return red + "✗ Not installed" + reset

	case "darwin":
		if !sysproxy.HasTool("launchctl") {
			return yellow + "⚠ Cannot check (launchctl not found)" + reset
		}
		cmd := exec.Command("launchctl", "list")
		output, err := cmd.Output()
		if err == nil && strings.Contains(string(output), "com.snirect.proxy") {
			return green + "✓ Running" + reset
		}
		return red + "✗ Not installed" + reset

	case "windows":
		if !sysproxy.HasTool("schtasks") {
			return yellow + "⚠ Cannot check (schtasks not found)" + reset
		}
		cmd := exec.Command("schtasks", "/Query", "/TN", "Snirect")
		_, err := cmd.Output()
		if err == nil {
			return green + "✓ Scheduled task exists" + reset
		}
		return red + "✗ Not installed" + reset

	default:
		return yellow + "⚠ Unknown platform" + reset
	}
}

func init() {
	RootCmd.AddCommand(statusCmd)
}
