package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/xihale/snirect/internal/service"
	"github.com/xihale/snirect/internal/sysproxy"
)

var certFirefox bool

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
		if err := service.SetupCA(false); err != nil {
			return fmt.Errorf("initialize CA failed: %w", err)
		}

		_, appDir, err := loadAppConfig()
		if err != nil {
			return err
		}
		p := certPath(appDir)

		installed, err := sysproxy.InstallCert(p)
		if err != nil {
			return fmt.Errorf("install failed: %w\n\nTry running with sudo or admin privileges", err)
		}
		if installed {
			fmt.Println("Root CA installed to system trust store")
		} else {
			fmt.Println("Root CA already installed in system trust store")
		}

		_ = sysproxy.InstallChromeCert(p)

		if certFirefox {
			if err := sysproxy.InstallFirefoxCert(p); err != nil {
				return fmt.Errorf("firefox cert install failed: %w", err)
			}
			fmt.Println("Root CA installed to Firefox profiles")
		} else {
			if _, err := exec.LookPath("certutil"); err == nil {
				if err := sysproxy.InstallFirefoxCert(p); err == nil {
					fmt.Println("Root CA installed to Firefox profiles")
				}
			}
		}

		return nil
	},
}

var certRemoveCmd = &cobra.Command{
	Use:     "uninstall",
	Aliases: []string{"remove"},
	Short:   "Remove root CA from system trust store",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, appDir, err := loadAppConfig()
		if err != nil {
			return err
		}
		p := certPath(appDir)
		if err := sysproxy.UninstallCert(p); err != nil {
			return fmt.Errorf("remove failed: %w\n\nYou may need to remove it manually via system certificate manager", err)
		}

		sysproxy.UninstallChromeCert()

		if err := sysproxy.UninstallFirefoxCert(); err != nil && certFirefox {
			return fmt.Errorf("firefox cert remove failed: %w", err)
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
			fmt.Printf("System CA: installed (%s)\n", p)
		} else {
			fmt.Printf("System CA: not installed (%s)\n", p)
			fmt.Println("Run: snirect cert install")
		}

		if sysproxy.HasFirefoxProfiles() {
			ffInstalled, err := sysproxy.CheckFirefoxCert()
			if err != nil {
				fmt.Printf("Firefox:   unknown (%v)\n", err)
			} else if ffInstalled {
				fmt.Println("Firefox:   installed")
			} else {
				fmt.Println("Firefox:   not installed")
			}
		}
		return nil
	},
}

func init() {
	certInstallCmd.Flags().BoolVar(&certFirefox, "firefox", false, "Install CA to Firefox profiles")
	certRemoveCmd.Flags().BoolVar(&certFirefox, "firefox", false, "Remove CA from Firefox profiles")

	certCmd.AddCommand(certInstallCmd, certRemoveCmd, certStatusCmd)
	firefoxCertCmd.Hidden = true
	certCmd.AddCommand(firefoxCertCmd)
}
