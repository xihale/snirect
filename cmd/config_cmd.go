package cmd

import (
	"fmt"

	"github.com/xihale/snirect/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	Long:  "Reset all configuration files to default values. Certificates and rules are preserved.",
	RunE: func(cmd *cobra.Command, args []string) error {
		appDir, err := config.EnsureConfig(true)
		if err != nil {
			return fmt.Errorf("reset failed: %w", err)
		}
		fmt.Printf("Configuration reset to defaults: %s\n", appDir)
		return nil
	},
}
