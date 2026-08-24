package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xihale/snirect/internal/service"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install binary and system service",
	Long: `Install snirect binary and register as system service.

  Linux:   ~/.local/bin + systemd user service
  macOS:   /usr/local/bin + launchd agent
  Windows: %LOCALAPPDATA%\Programs + Task Scheduler`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := service.Install(); err != nil {
			return err
		}
		fmt.Println("Installed successfully")
		fmt.Println("Next: snirect cert install && snirect -s")
		return nil
	},
}
