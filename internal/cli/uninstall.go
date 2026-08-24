package cli

import (
	"github.com/spf13/cobra"
	"github.com/xihale/snirect/internal/service"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove binary, service, config, and proxy settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		return service.Uninstall()
	},
}
