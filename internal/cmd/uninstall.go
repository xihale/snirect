package cmd

import (
	"snirect/internal/app"

	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall Snirect (remove binary, config, service, and cleanup)",
	Run: func(cmd *cobra.Command, args []string) {
		app.Uninstall()
	},
}

func init() {
	RootCmd.AddCommand(uninstallCmd)
}
