package cmd

import (
	"fmt"
	"path/filepath"
	"snirect/internal/config"

	"github.com/spf13/cobra"
)

var proxyEnvCmd = &cobra.Command{
	Use:   "proxy-env",
	Short: "Print shell export commands for proxy",
	Run: func(cmd *cobra.Command, args []string) {
		appDir, _ := config.GetAppDataDir()
		cfgPath := filepath.Join(appDir, "config.toml")
		cfg, err := config.LoadConfig(cfgPath)
		
		port := 7654 // Default fallback
		if err == nil {
			port = cfg.Server.Port
		}

		fmt.Printf("export http_proxy=http://127.0.0.1:%d\n", port)
		fmt.Printf("export https_proxy=http://127.0.0.1:%d\n", port)
	},
}

func init() {
	RootCmd.AddCommand(proxyEnvCmd)
}

