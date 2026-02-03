package cmd

import (
	"fmt"
	"snirect/internal/app"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:     "install",
	Aliases: []string{"i", "setup"},
	Short:   "Install snirect binary and service",
	Long: `Install snirect binary and set up as system service:
  - Linux: ~/.local/bin + systemd user service
  - macOS: /usr/local/bin + launchd service
  - Windows: %LOCALAPPDATA%\Programs\snirect + Task Scheduler

Note: CA certificate is not installed by this command.
It will be auto-generated on first run (snirect -s) or you can
manually install it with: snirect install-cert`,
	Example: `  snirect install          # Install binary and service
  snirect i                # Short alias
  snirect install --help   # More details`,
	Run: func(cmd *cobra.Command, args []string) {
		app.Install()
		fmt.Println("\n✓ Snirect installed successfully!")
		fmt.Println("\nNext steps:")
		fmt.Println("  1. snirect install-cert    # Install CA certificate (optional, will auto-install on first run)")
		fmt.Println("  2. snirect -s              # Start proxy with system proxy")
		fmt.Println("  3. snirect status          # Check installation status")
	},
}

func init() {
	RootCmd.AddCommand(installCmd)
}
