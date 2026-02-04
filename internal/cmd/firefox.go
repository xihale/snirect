package cmd

import (
	"fmt"
	"path/filepath"
	"snirect/internal/config"
	"snirect/internal/sysproxy"

	"github.com/spf13/cobra"
)

var firefoxCertCmd = &cobra.Command{
	Use:   "firefox-cert",
	Short: "管理 Firefox 系浏览器证书（需要 certutil）",
	Long: `将根 CA 证书安装到所有 Firefox 系浏览器的 profiles。

支持的浏览器：Firefox, Zen Browser, Waterfox, LibreWolf, Floorp

这些浏览器使用独立的 NSS 证书数据库，不读取系统信任库。
因此即使运行了 'snirect install-cert'，浏览器仍可能显示证书警告。

此命令会自动查找所有已安装浏览器的 profiles 并安装证书。

依赖：
  - Debian/Ubuntu: sudo apt install libnss3-tools
  - Fedora/RHEL:   sudo dnf install nss-tools
  - Arch Linux:    sudo pacman -S nss
  - macOS:         brew install nss

使用前请关闭 Firefox 浏览器。`,
	Example: `  snirect firefox-cert         # 安装证书到 Firefox
  snirect firefox-cert --check # 检查 Firefox 证书状态
  snirect firefox-cert --remove # 从 Firefox 移除证书`,
	RunE: func(cmd *cobra.Command, args []string) error {
		check, _ := cmd.Flags().GetBool("check")
		remove, _ := cmd.Flags().GetBool("remove")

		appDir, err := config.EnsureConfig(false)
		if err != nil {
			return fmt.Errorf("failed to init config: %w", err)
		}

		certPath := filepath.Join(appDir, "certs", "root.crt")

		if check {
			installed, err := sysproxy.CheckFirefoxCert()
			if err != nil {
				return fmt.Errorf("检查 Firefox 证书失败: %w", err)
			}
			if installed {
				fmt.Println("Snirect Root CA 已安装在 Firefox 中")
			} else {
				fmt.Println("Snirect Root CA 未安装在 Firefox 中")
				fmt.Println("\n运行以下命令安装: snirect firefox-cert")
			}
			return nil
		}

		if remove {
			if err := sysproxy.UninstallFirefoxCert(); err != nil {
				return fmt.Errorf("从 Firefox 移除证书失败: %w", err)
			}
			fmt.Println("证书已从 Firefox 移除")
			return nil
		}

		// Install
		fmt.Println("正在安装证书到 Firefox...")
		if err := sysproxy.InstallFirefoxCert(certPath); err != nil {
			return fmt.Errorf("安装证书到 Firefox 失败: %w\n\n可能的解决方案:\n  1. 确保已安装 certutil (libnss3-tools)\n  2. 关闭 Firefox 浏览器\n  3. 使用脚本: ./scripts/fix-firefox-cert.sh", err)
		}

		fmt.Println("\n证书安装成功！")
		fmt.Println("\n下一步:")
		fmt.Println("  1. 重启 Firefox 浏览器")
		fmt.Println("  2. 启动代理: snirect -s")
		fmt.Println("  3. 访问 https://www.google.com 测试")
		fmt.Println("\n验证证书:")
		fmt.Println("  Firefox → 设置 → 隐私与安全 → 证书 → 查看证书")
		fmt.Println("  搜索 'Snirect' 应该能看到 'Snirect Root CA'")
		return nil
	},
}

func init() {
	firefoxCertCmd.Flags().Bool("check", false, "检查 Firefox 证书安装状态")
	firefoxCertCmd.Flags().Bool("remove", false, "从 Firefox 移除证书")
	RootCmd.AddCommand(firefoxCertCmd)
}
