package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"snirect/internal/app"
	"snirect/internal/config"
	"snirect/internal/logger"
	"snirect/internal/sysproxy"
	"time"

	"github.com/spf13/cobra"
)

var installCertCmd = &cobra.Command{
	Use:     "install-cert",
	Aliases: []string{"install-ca", "ic"},
	Short:   "Install root CA to system trust store",
	Long: `Install the root CA certificate to system trust store.
This certificate is required for HTTPS proxying to work.

Platform-specific details:
  - Linux: Uses trust, update-ca-certificates, or update-ca-trust (requires sudo)
  - macOS: Uses security add-trusted-cert (requires confirmation)
  - Windows: Uses certutil -addstore (requires Administrator)

To check if already installed: snirect cert-status`,
	Example: `  snirect install-cert     # Install CA certificate
  snirect ic               # Short alias
  snirect cert-status      # Check if installed`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := app.SetupCA(true); err != nil {
			logger.Fatal("安装证书失败: %v\n\n请尝试以 sudo 或管理员权限运行。", err)
		}
		fmt.Println("✓ Root CA 安装成功！")
	},
}

var setProxyCmd = &cobra.Command{
	Use:     "set-proxy",
	Aliases: []string{"sp"},
	Short:   "Set system proxy PAC URL",
	Long: `Configure system-wide proxy settings using PAC (Proxy Auto-Config).
This tells your system to use Snirect for web browsing.

Platform-specific details:
  - Linux: gsettings (GNOME) or kwriteconfig5 (KDE)
  - macOS: networksetup (all network interfaces)
  - Windows: Registry (AutoConfigURL)

To verify proxy is set: snirect status
To remove proxy: snirect unset-proxy`,
	Example: `  snirect set-proxy        # Enable system proxy
  snirect sp               # Short alias
  snirect status           # Verify proxy is active`,
	Run: func(cmd *cobra.Command, args []string) {
		appDir, err := config.EnsureConfig(false)
		if err != nil {
			logger.Fatal("Failed to init config: %v", err)
		}
		cfg, _ := config.LoadConfig(filepath.Join(appDir, "config.toml"))
		pacURL := fmt.Sprintf("http://127.0.0.1:%d/pac/?t=%d", cfg.Server.Port, time.Now().Unix())
		sysproxy.SetPAC(pacURL)
		fmt.Println("✓ 系统代理配置成功。")
		fmt.Printf("  PAC 地址: %s\n", pacURL)
		fmt.Println("\n注意: 某些应用可能需要重启才能生效。")
	},
}

var unsetProxyCmd = &cobra.Command{
	Use:     "unset-proxy",
	Aliases: []string{"up"},
	Short:   "Clear system proxy settings",
	Long: `Remove system-wide proxy configuration.
Use this when you want to stop using Snirect or switch to another proxy.

This clears the PAC (Proxy Auto-Config) URL from:
  - Linux: gsettings (GNOME) or kwriteconfig5 (KDE)
  - macOS: networksetup (all network interfaces)
  - Windows: Registry (AutoConfigURL)`,
	Example: `  snirect unset-proxy      # Disable system proxy
  snirect up               # Short alias`,
	Run: func(cmd *cobra.Command, args []string) {
		sysproxy.ClearPAC()
		fmt.Println("✓ 系统代理已清除。")
		fmt.Println("您的系统现在将直接连接到互联网。")
	},
}

var resetConfigCmd = &cobra.Command{
	Use:   "reset-config",
	Short: "Reset configuration files to defaults",
	Long: `Reset all configuration files to their default values.
WARNING: This will overwrite your custom settings!

Files affected:
  - config.toml (main configuration)
  - rules.toml (SNI bypass rules)
  - pac (proxy auto-config script)

Your certificates in the certs/ directory will NOT be affected.`,
	Example: `  snirect reset-config       # Reset to defaults`,
	Run: func(cmd *cobra.Command, args []string) {
		appDir, err := config.EnsureConfig(true)
		if err != nil {
			logger.Fatal("重置配置失败: %v", err)
		}
		fmt.Println("✓ 配置文件已重置为默认值。")
		fmt.Printf("  配置目录: %s\n", appDir)
		fmt.Println("\n注意: 您的证书文件 (certs/ 目录下) 已被保留。")
	},
}

var certStatusCmd = &cobra.Command{
	Use:     "cert-status",
	Aliases: []string{"ca-status", "cs"},
	Short:   "Check if root CA is installed in system trust store",
	Long: `Check the installation status of the Snirect root CA certificate.
This helps diagnose HTTPS connectivity issues.

Checks:
  - Linux: /usr/local/share/ca-certificates/, /etc/pki/ca-trust/, and trust store
  - macOS: login.keychain for "Snirect Root CA"
  - Windows: User certificate store (Root) for "Snirect Root CA"`,
	Example: `  snirect cert-status      # Check CA installation
  snirect cs               # Short alias`,
	Run: func(cmd *cobra.Command, args []string) {
		appDir, err := config.EnsureConfig(false)
		if err != nil {
			logger.Fatal("Failed to init config: %v\n\nEnsure you have write permissions to the config directory.", err)
		}

		certDir := filepath.Join(appDir, "certs")
		caCertPath := filepath.Join(certDir, "root.crt")

		// Check if cert file exists
		if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
			fmt.Printf("Certificate file not found: %s\n\n", caCertPath)
			fmt.Println("Run 'snirect' first to generate the certificate.")
			return
		}

		installed, err := sysproxy.CheckCertStatus(caCertPath)
		if err != nil {
			logger.Fatal("Failed to check certificate status: %v", err)
		}

		if installed {
			fmt.Println("状态: 根 CA 已安装在系统信任库中")
			fmt.Printf("证书路径: %s\n", caCertPath)
			fmt.Println("\n✓ 浏览器现在应该信任 Snirect 的 HTTPS 证书。")
		} else {
			fmt.Println("状态: 根 CA 尚未安装在系统信任库中")
			fmt.Printf("证书路径: %s\n", caCertPath)
			fmt.Println("\n⚠ 浏览器访问 HTTPS 网站时会显示证书警告。")
			fmt.Println("\n要进行安装，请运行: snirect install-cert")
		}
	},
}

var uninstallCertCmd = &cobra.Command{
	Use:     "uninstall-cert",
	Aliases: []string{"uninstall-ca", "uc"},
	Short:   "Remove root CA from system trust store",
	Long: `Remove the Snirect root CA certificate from system trust store.
Use this when uninstalling Snirect or if you want to stop trusting its certificates.

Platform-specific details:
  - Linux: Removes from /usr/local/share/ca-certificates/, /etc/pki/ca-trust/, and trust store
  - macOS: Removes from login.keychain
  - Windows: Removes from user certificate store (Root)

To check current status: snirect cert-status`,
	Example: `  snirect uninstall-cert   # Remove CA certificate
  snirect uc               # Short alias`,
	Run: func(cmd *cobra.Command, args []string) {
		appDir, err := config.EnsureConfig(false)
		if err != nil {
			logger.Fatal("Failed to init config: %v", err)
		}

		certDir := filepath.Join(appDir, "certs")
		caCertPath := filepath.Join(certDir, "root.crt")

		if err := sysproxy.UninstallCert(caCertPath); err != nil {
			logger.Fatal("卸载证书失败: %v\n\n您可能需要手动通过系统的证书管理器将其删除。", err)
		}
		fmt.Println("✓ 根 CA 已从系统信任库中移除。")
	},
}

func init() {
	RootCmd.AddCommand(installCertCmd)
	RootCmd.AddCommand(setProxyCmd)
	RootCmd.AddCommand(unsetProxyCmd)
	RootCmd.AddCommand(resetConfigCmd)
	RootCmd.AddCommand(certStatusCmd)
	RootCmd.AddCommand(uninstallCertCmd)
}
