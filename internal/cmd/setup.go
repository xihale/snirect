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
	Short:   "安装根 CA 到系统信任库",
	Long: `将根 CA 证书安装到系统信任库中。
这是实现 HTTPS 代理所必需的步骤。

平台特定细节：
  - Linux: 使用 trust, update-ca-certificates 或 update-ca-trust (需要 sudo)
  - macOS: 使用 security add-trusted-cert (需要确认)
  - Windows: 使用 certutil -addstore (需要管理员权限)

查看安装状态：snirect cert-status`,
	Example: `  snirect install-cert     # 安装 CA 证书
  snirect ic               # 简写
  snirect cert-status      # 检查是否已安装`,
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
	Short:   "设置系统代理 PAC 地址",
	Long: `使用 PAC (代理自动配置) 配置系统范围的代理设置。
这会告诉您的系统使用 Snirect 进行网页浏览。

平台特定细节：
  - Linux: gsettings (GNOME) 或 kwriteconfig5 (KDE)
  - macOS: networksetup (所有网络接口)
  - Windows: 注册表 (AutoConfigURL)

验证代理是否设置：snirect status
清除代理：snirect unset-proxy`,
	Example: `  snirect set-proxy        # 启用系统代理
  snirect sp               # 简写
  snirect status           # 验证代理是否活跃`,
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
	Short:   "清除系统代理设置",
	Long: `移除系统范围的代理配置。
当您想停止使用 Snirect 或切换到其他代理时使用此命令。

这将从以下位置清除 PAC (代理自动配置) 地址：
  - Linux: gsettings (GNOME) 或 kwriteconfig5 (KDE)
  - macOS: networksetup (所有网络接口)
  - Windows: 注册表 (AutoConfigURL)`,
	Example: `  snirect unset-proxy      # 禁用系统代理
  snirect up               # 简写`,
	Run: func(cmd *cobra.Command, args []string) {
		sysproxy.ClearPAC()
		fmt.Println("✓ 系统代理已清除。")
		fmt.Println("您的系统现在将直接连接到互联网。")
	},
}

var resetConfigCmd = &cobra.Command{
	Use:   "reset-config",
	Short: "将配置文件重置为默认值",
	Long: `将所有配置文件重置为默认值。
警告：这将覆盖您的自定义设置！

受影响的文件：
  - config.toml (主配置)
  - rules.toml (SNI 分流规则)
  - pac (代理自动配置脚本)

您的证书文件 (certs/ 目录下) 不会受到影响。`,
	Example: `  snirect reset-config       # 重置为默认设置`,
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
	Short:   "检查根 CA 是否已安装在系统信任库中",
	Long: `检查 Snirect 根 CA 证书的安装状态。
这有助于诊断 HTTPS 连接问题。

检查项：
  - Linux: /usr/local/share/ca-certificates/, /etc/pki/ca-trust/ 以及信任库
  - macOS: 登录钥匙串中的 "Snirect Root CA"
  - Windows: 用户证书存储 (Root) 中的 "Snirect Root CA"`,
	Example: `  snirect cert-status      # 检查 CA 安装状态
  snirect cs               # 简写`,
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
	Short:   "从系统信任库移除根 CA",
	Long: `从系统信任库中移除 Snirect 根 CA 证书。
当您卸载 Snirect 或想停止信任其证书时使用此命令。

平台特定细节：
  - Linux: 从 /usr/local/share/ca-certificates/, /etc/pki/ca-trust/ 和信任库中移除
  - macOS: 从登录钥匙串中移除
  - Windows: 从用户证书存储 (Root) 中移除

查看当前状态：snirect cert-status`,
	Example: `  snirect uninstall-cert   # 移除 CA 证书
  snirect uc               # 简写`,
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
