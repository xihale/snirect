package cmd

import (
	"fmt"
	"path/filepath"
	"runtime"
	"snirect/internal/config"

	"github.com/spf13/cobra"
)

var proxyEnvCmd = &cobra.Command{
	Use:   "proxy-env",
	Short: "Print shell export commands for proxy",
	Long: `Print environment variable commands for configuring proxy in the current terminal.
This is useful when you want to use Snirect only in the current shell session
without changing system-wide proxy settings.

Usage examples:
  Linux/macOS: eval $(snirect proxy-env)
  Windows CMD: FOR /F %i IN ('snirect proxy-env') DO %i
  PowerShell:  & snirect proxy-env | Invoke-Expression`,
	Example: `  snirect proxy-env        # Print proxy environment commands
  eval $(snirect proxy-env)  # Apply to current shell (Linux/macOS)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appDir, err := config.GetAppDataDir()
		if err != nil {
			return fmt.Errorf("failed to get app data dir: %w", err)
		}
		cfgPath := filepath.Join(appDir, "config.toml")
		cfg, err := config.LoadConfig(cfgPath)

		port := 7654
		if err == nil && cfg != nil {
			port = cfg.Server.Port
		}

		fmt.Println("# Run these commands to set proxy for current shell:")
		if runtime.GOOS == "windows" {
			fmt.Printf("set http_proxy=http://127.0.0.1:%d\n", port)
			fmt.Printf("set https_proxy=http://127.0.0.1:%d\n", port)
		} else {
			fmt.Printf("export http_proxy=http://127.0.0.1:%d\n", port)
			fmt.Printf("export https_proxy=http://127.0.0.1:%d\n", port)
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(proxyEnvCmd)
}
