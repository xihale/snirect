package cmd

import (
	"fmt"
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
	Aliases: []string{"install-ca", "I", "C"},
	Short:   "Install root CA to system trust store",
	Long: `Install the root CA certificate to system trust store:
  - Linux: trust/update-ca-certificates/update-ca-trust
  - macOS: security add-trusted-cert (may require confirmation)
  - Windows: certutil -addstore (may require administrator)`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := app.SetupCA(true); err != nil {
			logger.Fatal("Failed: %v", err)
		}
	},
}

var setProxyCmd = &cobra.Command{
	Use:     "set-proxy",
	Aliases: []string{"P"},
	Short:   "Set system proxy PAC URL",
	Long: `Configure system-wide proxy settings:
  - Linux: gsettings (GNOME) or kwriteconfig5 (KDE)
  - macOS: networksetup (all network interfaces)
  - Windows: Registry (AutoConfigURL)`,
	Run: func(cmd *cobra.Command, args []string) {
		appDir, err := config.EnsureConfig(false)
		if err != nil {
			logger.Fatal("Failed to init config: %v", err)
		}
		cfg, _ := config.LoadConfig(filepath.Join(appDir, "config.toml"))
		pacURL := fmt.Sprintf("http://127.0.0.1:%d/pac/?t=%d", cfg.Server.Port, time.Now().Unix())
		sysproxy.SetPAC(pacURL)
	},
}

var unsetProxyCmd = &cobra.Command{
	Use:     "unset-proxy",
	Aliases: []string{"U"},
	Short:   "Clear system proxy settings",
	Run: func(cmd *cobra.Command, args []string) {
		sysproxy.ClearPAC()
	},
}

var resetConfigCmd = &cobra.Command{
	Use:   "reset-config",
	Short: "Force reset configuration files to defaults and exit",
	Run: func(cmd *cobra.Command, args []string) {
		if _, err := config.EnsureConfig(true); err != nil {
			logger.Fatal("Failed to reset config: %v", err)
		}
		fmt.Println("Configuration reset to defaults.")
	},
}

func init() {
	RootCmd.AddCommand(installCertCmd)
	RootCmd.AddCommand(setProxyCmd)
	RootCmd.AddCommand(unsetProxyCmd)
	RootCmd.AddCommand(resetConfigCmd)
}
