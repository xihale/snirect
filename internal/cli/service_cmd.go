package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/xihale/snirect/internal/service"
	"github.com/xihale/snirect/internal/sysproxy"
)

var svcSetProxy bool

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage user background service",
}

var serviceInstallCmd = &cobra.Command{
	Use:     "install",
	Aliases: []string{"enable"},
	Short:   "Install and start background service",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := service.Install(service.Options{SetProxy: svcSetProxy}); err != nil {
			return err
		}
		fmt.Println("Service installed and started")

		_, appDir, err := loadAppConfig()
		if err == nil {
			cp := certPath(appDir)
			if _, err := os.Stat(cp); err != nil {
				fmt.Println("Next: snirect cert install")
			} else if ok, _ := sysproxy.CheckCertStatus(cp); !ok {
				fmt.Println("Next: snirect cert install")
			}
		}
		return nil
	},
}

var serviceUninstallCmd = &cobra.Command{
	Use:     "uninstall",
	Aliases: []string{"remove", "disable"},
	Short:   "Stop and remove background service",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := service.Uninstall(); err != nil {
			return err
		}
		fmt.Println("Service uninstalled")
		return nil
	},
}

var serviceStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start background service",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := service.Start(); err != nil {
			return err
		}
		fmt.Println("Service started")
		return nil
	},
}

var serviceStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop background service",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := service.Stop(); err != nil {
			return err
		}
		fmt.Println("Service stopped")
		return nil
	},
}

var serviceRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart background service",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := service.Restart(); err != nil {
			return err
		}
		fmt.Println("Service restarted")
		return nil
	},
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show background service status",
	Run: func(cmd *cobra.Command, args []string) {
		installed, running, detail := service.Status()
		switch {
		case running:
			fmt.Println("Service: running")
		case installed:
			if detail != "" {
				fmt.Printf("Service: installed (%s)\n", detail)
			} else {
				fmt.Println("Service: installed (stopped)")
			}
		default:
			if detail != "" {
				fmt.Printf("Service: not installed (%s)\n", detail)
			} else {
				fmt.Println("Service: not installed")
			}
		}
	},
}

func init() {
	serviceInstallCmd.Flags().BoolVarP(&svcSetProxy, "set-proxy", "s", false, "Set system proxy on start")

	serviceCmd.AddCommand(
		serviceInstallCmd,
		serviceUninstallCmd,
		serviceStartCmd,
		serviceStopCmd,
		serviceRestartCmd,
		serviceStatusCmd,
	)
}
