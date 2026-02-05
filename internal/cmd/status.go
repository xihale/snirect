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

	fmt.Printf("\n%s%sSnirect 运行状态%s\n", bold, cyan, reset)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// 1. Check configuration
	appDir, err := config.GetAppDataDir()
	if err != nil {
		logger.Warn("获取配置目录失败: %v", err)
		appDir = "未知"
	}

	configPath := filepath.Join(appDir, "config.toml")
	cfg, cfgErr := config.LoadConfig(configPath)

	fmt.Printf("%s配置信息:%s\n", bold, reset)
	fmt.Printf("  配置目录: %s%s%s\n", cyan, appDir, reset)
	if cfgErr != nil {
		fmt.Printf("  配置状态: %s%s%s (正在使用默认设置)\n", yellow, "[!] 加载失败", reset)
	} else {
		fmt.Printf("  配置状态: %s%s%s\n", green, "[+] 已加载", reset)
		fmt.Printf("  服务器端口: %s%d%s\n", cyan, cfg.Server.Port, reset)
	}
	fmt.Println()

	// 2. Check certificate
	certDir := filepath.Join(appDir, "certs")
	caCertPath := filepath.Join(certDir, "root.crt")

	fmt.Printf("%s证书信息:%s\n", bold, reset)
	if _, err := os.Stat(caCertPath); err == nil {
		fmt.Printf("  CA 文件: %s%s%s\n", green, "[+] 已存在", reset)
		fmt.Printf("  路径: %s%s%s\n", cyan, caCertPath, reset)

		// Check if installed in system
		installed, err := sysproxy.CheckCertStatus(caCertPath)
		if err != nil {
			fmt.Printf("  系统信任: %s%s%s (%v)\n", yellow, "[?] 未知", reset, err)
		} else if installed {
			fmt.Printf("  系统信任: %s%s%s\n", green, "[+] 已安装", reset)
		} else {
			fmt.Printf("  系统信任: %s%s%s\n", red, "[-] 未安装", reset)
			fmt.Printf("    请运行: %ssnirect install-cert%s\n", yellow, reset)
		}
	} else {
		fmt.Printf("  CA 文件: %s%s%s\n", red, "[-] 未找到", reset)
		fmt.Printf("    请运行: %ssnirect%s 以生成证书\n", yellow, reset)
	}
	fmt.Println()

	// 3. Check proxy
	fmt.Printf("%s系统代理:%s\n", bold, reset)
	if isProxySet() {
		fmt.Printf("  状态: %s%s%s\n", green, "[+] 已启用", reset)
		if cfg != nil {
			pacURL := fmt.Sprintf("http://127.0.0.1:%d/pac/", cfg.Server.Port)
			fmt.Printf("  PAC 地址: %s%s%s\n", cyan, pacURL, reset)
		}
	} else {
		fmt.Printf("  状态: %s%s%s\n", red, "[-] 未设置", reset)
		fmt.Printf("    请运行: %ssnirect set-proxy%s\n", yellow, reset)
	}
	fmt.Println()

	// 4. Check service
	fmt.Printf("%s服务状态:%s\n", bold, reset)
	serviceStatus := checkServiceStatus()
	fmt.Printf("  状态: %s\n", serviceStatus)
	fmt.Println()

	// 5. Check modules
	fmt.Printf("%s组件模块:%s\n", bold, reset)
	for _, m := range getModuleStatus() {
		status := red + "[-] " + reset
		if m.Enabled {
			status = green + "[+] " + reset
		}
		fmt.Printf("  %s%s\n", status, m.Name)
	}
	fmt.Println()

	// 6. Quick tips
	fmt.Printf("%s%s快速命令:%s\n", bold, cyan, reset)
	fmt.Printf("  启动代理:      %ssnirect%s\n", yellow, reset)
	fmt.Printf("  安装 CA:       %ssnirect install-cert%s\n", yellow, reset)
	fmt.Printf("  设置代理:      %ssnirect set-proxy%s\n", yellow, reset)

	// Log location logic
	logHint := ""
	if cfg != nil && cfg.Log.File != "" {
		// 1. Priority: User configured log file
		absLogPath, _ := filepath.Abs(cfg.Log.File)
		logHint = fmt.Sprintf("tail -f %s", absLogPath)
		if runtime.GOOS == "windows" {
			logHint = fmt.Sprintf("Get-Content -Wait \"%s\"", absLogPath)
		}
	} else {
		// 2. Default behavior (Stdout/Stderr)
		// Depends on how it's running
		if strings.Contains(serviceStatus, "正在运行") {
			switch runtime.GOOS {
			case "linux":
				logHint = "journalctl --user -u snirect -f"
			case "darwin":
				homeDir, _ := os.UserHomeDir()
				logHint = fmt.Sprintf("tail -f %s/Library/Logs/snirect.log", homeDir)
			case "windows":
				logHint = "查看 Windows 事件查看器 或 确保配置了 logfile"
			}
		} else {
			logHint = "直接运行可以看到日志 (默认输出到控制台)"
		}
	}

	fmt.Printf("  查看日志:      %s%s%s\n", yellow, logHint, reset)
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
			return yellow + "[!] 无法检查 (未找到 systemctl)" + reset
		}
		cmd := exec.Command("systemctl", "--user", "is-active", "snirect")
		output, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(output)) == "active" {
			return green + "[+] 正在运行" + reset
		}
		cmd = exec.Command("systemctl", "--user", "is-enabled", "snirect")
		output, _ = cmd.Output()
		if strings.TrimSpace(string(output)) == "enabled" {
			return yellow + "[*] 已启用 (但未运行)" + reset
		}
		return red + "[-] 未安装" + reset

	case "darwin":
		if !sysproxy.HasTool("launchctl") {
			return yellow + "[!] 无法检查 (未找到 launchctl)" + reset
		}
		cmd := exec.Command("launchctl", "list")
		output, err := cmd.Output()
		if err == nil && strings.Contains(string(output), "com.snirect.proxy") {
			return green + "[+] 正在运行" + reset
		}
		return red + "[-] 未安装" + reset

	case "windows":
		if !sysproxy.HasTool("schtasks") {
			return yellow + "[!] 无法检查 (未找到 schtasks)" + reset
		}
		cmd := exec.Command("schtasks", "/Query", "/TN", "Snirect")
		_, err := cmd.Output()
		if err == nil {
			return green + "[+] 已创建计划任务" + reset
		}
		return red + "[-] 未安装" + reset

	default:
		return yellow + "[!] 未知平台" + reset
	}
}

func init() {
	RootCmd.AddCommand(statusCmd)
}
