package cmd

import (
	"snirect/internal/app"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install snirect binary, service, and root certificate",
	Long: `Install snirect to your system:
  - Linux: ~/.local/bin + systemd service
  - macOS: /usr/local/bin + launchd service
  - Windows: %LOCALAPPDATA%\Programs\snirect + Task Scheduler

Automatically installs Root CA to system trust store.`,
	Run: func(cmd *cobra.Command, args []string) {
		app.Install()
	},
}

func init() {
	RootCmd.AddCommand(installCmd)
}
