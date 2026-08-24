package cmd

import (
	"github.com/xihale/snirect/service"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove binary, service, config, and proxy settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		return service.Uninstall()
	},
}
