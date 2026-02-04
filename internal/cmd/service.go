package cmd

import (
	"fmt"
	"snirect/internal/app"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:     "install",
	Aliases: []string{"i", "setup"},
	Short:   "Install snirect binary and service",
	Long: `Install snirect binary and set up as system service:
  - Linux: ~/.local/bin + systemd user service
  - macOS: /usr/local/bin + launchd service
  - Windows: %LOCALAPPDATA%\Programs\snirect + Task Scheduler

Note: CA certificate is not installed by this command.
It will be auto-generated on first run (snirect -s) or you can
manually install it with: snirect install-cert`,
	Example: `  snirect install          # Install binary and service
  snirect i                # Short alias
  snirect install --help   # More details`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := app.Install(); err != nil {
			return err
		}
		fmt.Println("\nSnirect 安装成功。")
		fmt.Println("\n后续步骤:")
		fmt.Println("  1. snirect install-cert    # 安装 CA 证书 (可选，首次运行会自动安装)")
		fmt.Println("  2. snirect -s              # 启动代理并自动设置系统代理")
		fmt.Println("  3. snirect status          # 检查安装状态")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(installCmd)
}