package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"snirect/internal/config"
	"snirect/internal/logger"
	"snirect/internal/update"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update snirect to the latest version",
	Long: `Downloads the latest version of snirect from GitHub releases and installs it.
This will download the binary for your platform and prepare for self-update.`,
	Example: `  snirect update`,
	RunE: func(cmd *cobra.Command, args []string) error {
		currentVersion := Version
		fmt.Printf("Current version: %s\n", currentVersion)
		logger.Info("Starting update check for version %s", currentVersion)

		// Use manager with internal network stack for update check
		appDir, err := config.EnsureConfig(false)
		if err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}

		configPath := filepath.Join(appDir, "config.toml")
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		logger.SetLevel(cfg.Log.Level)
		mgr := update.NewManager(cfg, &config.Rules{})
		var hasUpdate bool
		var latestVersion string
		logger.Info("Checking for updates via GitHub API...")
		hasUpdate, latestVersion, err = mgr.CheckForUpdate(currentVersion)
		if err != nil {
			return fmt.Errorf("failed to check for updates: %w", err)
		}

		if !hasUpdate {
			fmt.Printf("[+] Already up to date (remote: %s)\n", latestVersion)
			logger.Info("No update available (current: %s, latest: %s)", currentVersion, latestVersion)
			return nil
		}

		logger.Info("Update found: %s -> %s", currentVersion, latestVersion)
		fmt.Printf("New version available: %s\n", latestVersion)
		fmt.Println("Downloading...")

		rulesPath := filepath.Join(appDir, "rules.toml")
		rules, err := config.LoadRules(rulesPath)
		if err != nil {
			logger.Warn("Failed to load rules.toml: %v. Using defaults.", err)
			rules = &config.Rules{}
		}

		// Reuse the same manager with loaded rules
		mgr = update.NewManager(cfg, rules)

		downloadDir := mgr.GetTempDownloadDir(appDir)
		assetName := fmt.Sprintf("snirect-%s-%s", runtime.GOOS, runtime.GOARCH)
		if runtime.GOOS == "windows" {
			assetName += ".exe"
		}
		downloadPath := filepath.Join(downloadDir, assetName)

		downloadURL := fmt.Sprintf("https://github.com/xihale/snirect/releases/download/%s/%s", latestVersion, assetName)
		logger.Info("Downloading from: %s", downloadURL)
		if err := mgr.DownloadFile(downloadPath, downloadURL); err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
		logger.Info("Download completed: %s", downloadPath)

		if runtime.GOOS != "windows" {
			if err := os.Chmod(downloadPath, 0755); err != nil {
				return fmt.Errorf("failed to set permissions: %w", err)
			}
			logger.Info("Set executable permissions")
		}

		fmt.Printf("Downloaded: %s\n", downloadPath)
		fmt.Println("Preparing self-update...")

		if err := mgr.MarkForSelfUpdate(appDir, latestVersion); err != nil {
			return fmt.Errorf("failed to mark update: %w", err)
		}

		if err := mgr.UpdateCheckTimestamp(appDir); err != nil {
			logger.Warn("Failed to update check timestamp: %v", err)
		}

		fmt.Println("[+] Update prepared!")
		fmt.Println("Restart snirect to complete the update.")
		logger.Info("Update preparation complete. Restart required.")

		return nil
	},
}

func init() {
	RootCmd.AddCommand(updateCmd)
}
