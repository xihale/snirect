package cmd

import (
	"snirect/internal/app"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install snirect binary, systemd service, and root certificate",
	Run: func(cmd *cobra.Command, args []string) {
		app.Install()
	},
}

func init() {
	RootCmd.AddCommand(installCmd)
}
