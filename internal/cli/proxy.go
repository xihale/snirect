package cli

import (
	"fmt"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/xihale/snirect/internal/sysproxy"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Manage system proxy settings",
}

var proxySetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set system proxy PAC",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := loadAppConfig()
		if err != nil {
			return err
		}
		pacURL := fmt.Sprintf("http://127.0.0.1:%d/pac/?t=%d", cfg.Server.Port, time.Now().Unix())
		sysproxy.SetPAC(pacURL)
		fmt.Printf("System proxy set: %s\n", pacURL)
		return nil
	},
}

var proxyUnsetCmd = &cobra.Command{
	Use:   "unset",
	Short: "Clear system proxy settings",
	Run: func(cmd *cobra.Command, args []string) {
		sysproxy.ClearPAC()
		fmt.Println("System proxy cleared")
	},
}

var proxyEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "Print shell export commands for proxy",
	Long: `Print environment variable commands for current shell proxy setup.

  Linux/macOS: eval $(snirect proxy env)
  Windows CMD: FOR /F %i IN ('snirect proxy env') DO %i`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := loadAppConfig()
		if err != nil {
			return err
		}
		addr := fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.Port)
		if runtime.GOOS == "windows" {
			fmt.Printf("set http_proxy=%s\n", addr)
			fmt.Printf("set https_proxy=%s\n", addr)
		} else {
			fmt.Printf("export http_proxy=%s\n", addr)
			fmt.Printf("export https_proxy=%s\n", addr)
		}
		return nil
	},
}
