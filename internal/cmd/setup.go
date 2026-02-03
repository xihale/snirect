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
			logger.Fatal("Failed to install certificate: %v\n\nTry running with sudo/administrator privileges.", err)
		}
		fmt.Println("✓ Root CA installed successfully!")
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
		fmt.Println("✓ System proxy configured.")
		fmt.Printf("  PAC URL: %s\n", pacURL)
		fmt.Println("\nNote: Some applications may need to be restarted.")
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
		fmt.Println("✓ System proxy cleared.")
		fmt.Println("Your system will now connect directly to the internet.")
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
			logger.Fatal("Failed to reset config: %v", err)
		}
		fmt.Println("✓ Configuration reset to defaults.")
		fmt.Printf("  Config location: %s\n", appDir)
		fmt.Println("\nNote: Your certificates in certs/ were preserved.")
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
			fmt.Println("Status: Root CA is installed in system trust store")
			fmt.Printf("Certificate path: %s\n", caCertPath)
			fmt.Println("\n✓ Browsers should trust Snirect's HTTPS certificates.")
		} else {
			fmt.Println("Status: Root CA is NOT installed in system trust store")
			fmt.Printf("Certificate path: %s\n", caCertPath)
			fmt.Println("\n⚠ Browsers will show certificate warnings for HTTPS sites.")
			fmt.Println("\nTo install, run: snirect install-cert")
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
			logger.Fatal("Failed to uninstall certificate: %v\n\nYou may need to manually remove it via your system's certificate manager.", err)
		}
		fmt.Println("✓ Root CA removed from system trust store.")
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
