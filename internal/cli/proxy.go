package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/xihale/snirect/internal/sysproxy"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Manage system proxy settings",
}

var proxyOnCmd = &cobra.Command{
	Use:     "on",
	Aliases: []string{"set"},
	Short:   "Enable system PAC proxy",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := loadAppConfig()
		if err != nil {
			return err
		}
		url := pacURL("127.0.0.1", cfg.Server.Port)
		sysproxy.SetPAC(url)
		fmt.Printf("System proxy enabled: %s\n", url)
		return nil
	},
}

var proxyOffCmd = &cobra.Command{
	Use:     "off",
	Aliases: []string{"unset", "clear"},
	Short:   "Disable system proxy settings",
	Run: func(cmd *cobra.Command, args []string) {
		sysproxy.ClearPAC()
		fmt.Println("System proxy disabled")
	},
}

var proxyStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show system proxy status",
	Run: func(cmd *cobra.Command, args []string) {
		if sysproxy.IsSystemProxySet() {
			fmt.Println("System proxy: enabled")
		} else {
			fmt.Println("System proxy: disabled")
		}
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

func init() {
	proxyCmd.AddCommand(proxyOnCmd, proxyOffCmd, proxyStatusCmd, proxyEnvCmd)
}
