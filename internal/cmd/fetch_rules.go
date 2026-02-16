package cmd

import (
	"fmt"
	"path/filepath"

	"snirect/internal/config"
	"snirect/internal/update"

	"github.com/spf13/cobra"
)

var fetchRulesCmd = &cobra.Command{
	Use:   "fetch-rules",
	Short: "Fetch and update rules from upstream",
	Long: `Downloads the latest rules from the upstream source (Ceiling-Host) and updates them locally.
This will replace the current rules with the latest version.

Rules are fetched using Snirect's internal network stack.`,
	Example: `  snirect fetch-rules`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appDir, err := config.EnsureConfig(false)
		if err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}

		configPath := filepath.Join(appDir, "config.toml")
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		mgr := update.NewManager(cfg, &config.Rules{})
		if err := mgr.FetchRules(appDir); err != nil {
			return fmt.Errorf("fetch rules failed: %w", err)
		}

		fmt.Println("[+] Rules updated successfully!")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(fetchRulesCmd)
}
