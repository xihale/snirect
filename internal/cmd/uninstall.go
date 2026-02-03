package cmd

import (
	"snirect/internal/app"

	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:     "uninstall",
	Aliases: []string{"rm", "remove"},
	Short:   "Uninstall Snirect (remove binary, config, service, and cleanup)",
	Long: `Completely remove Snirect from your system:
  - Stops and removes the service
  - Removes the binary from system PATH
  - Deletes configuration directory
  - Clears system proxy settings
  - Removes shell completions`,
	Example: `  snirect uninstall        # Full removal
  snirect rm               # Short alias`,
	Run: func(cmd *cobra.Command, args []string) {
		app.Uninstall()
	},
}

func init() {
	RootCmd.AddCommand(uninstallCmd)
}
