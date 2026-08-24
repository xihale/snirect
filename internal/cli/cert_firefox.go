package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xihale/snirect/internal/sysproxy"
)

var firefoxCertCmd = &cobra.Command{
	Use:   "firefox",
	Short: "Manage Firefox NSS certificate",
	Long: `Manage root CA in Firefox NSS database.

Firefox uses its own certificate store (NSS), separate from the system trust store.
Requires certutil: apt install libnss3-tools / brew install nss

Close Firefox before running these commands.`,
}

var firefoxInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install CA to Firefox profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, appDir, err := loadAppConfig()
		if err != nil {
			return err
		}
		if err := sysproxy.InstallFirefoxCert(certPath(appDir)); err != nil {
			return fmt.Errorf("install failed: %w\n\nEnsure certutil is installed and Firefox is closed", err)
		}
		fmt.Println("Certificate installed to Firefox. Restart Firefox to apply.")
		return nil
	},
}

var firefoxRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove CA from Firefox profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := sysproxy.UninstallFirefoxCert(); err != nil {
			return fmt.Errorf("remove failed: %w", err)
		}
		fmt.Println("Certificate removed from Firefox")
		return nil
	},
}

var firefoxStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check Firefox certificate status",
	RunE: func(cmd *cobra.Command, args []string) error {
		installed, err := sysproxy.CheckFirefoxCert()
		if err != nil {
			return fmt.Errorf("check failed: %w", err)
		}
		if installed {
			fmt.Println("Firefox: installed")
		} else {
			fmt.Println("Firefox: not installed")
			fmt.Println("Run: snirect cert firefox install")
		}
		return nil
	},
}
