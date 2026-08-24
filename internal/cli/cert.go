package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/xihale/snirect/internal/service"
	"github.com/xihale/snirect/internal/sysproxy"
)

var certCmd = &cobra.Command{
	Use:   "cert",
	Short: "Manage CA certificate",
}

var certInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install root CA to system trust store",
	Long: `Install the root CA certificate to the system trust store.

Platform details:
  Linux:   trust (p11-kit) preferred; falls back to distro anchor dirs via
           update-ca-trust / update-ca-certificates; requires sudo
  macOS:   security add-trusted-cert
  Windows: certutil -addstore (requires admin)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := service.SetupCA(true); err != nil {
			return fmt.Errorf("install failed: %w\n\nTry running with sudo or admin privileges", err)
		}
		fmt.Println("Root CA installed successfully")
		return nil
	},
}

var certRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove root CA from system trust store",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, appDir, err := loadAppConfig()
		if err != nil {
			return err
		}
		if err := sysproxy.UninstallCert(certPath(appDir)); err != nil {
			return fmt.Errorf("remove failed: %w\n\nYou may need to remove it manually via system certificate manager", err)
		}
		fmt.Println("Root CA removed from system trust store")
		return nil
	},
}

var certStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check root CA installation status",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, appDir, err := loadAppConfig()
		if err != nil {
			return err
		}
		p := certPath(appDir)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			fmt.Printf("Certificate file not found: %s\n", p)
			fmt.Println("Run 'snirect' first to generate the certificate")
			return nil
		}
		installed, err := sysproxy.CheckCertStatus(p)
		if err != nil {
			return fmt.Errorf("check failed: %w", err)
		}
		if installed {
			fmt.Printf("Status: installed (%s)\n", p)
		} else {
			fmt.Printf("Status: not installed (%s)\n", p)
			fmt.Println("Run: snirect cert install")
		}
		return nil
	},
}
