package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
	"github.com/xihale/snirect/internal/color"
	"github.com/xihale/snirect/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print effective TOML configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := loadAppConfig()
		if err != nil {
			return err
		}
		printConfig(cfg)
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print path to config.toml",
	RunE: func(cmd *cobra.Command, args []string) error {
		if configFile != "" {
			fmt.Println(configFile)
			return nil
		}
		appDir, err := config.GetAppDataDir()
		if err != nil {
			return err
		}
		fmt.Println(filepath.Join(appDir, "config.toml"))
		return nil
	},
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	Long:  "Reset configuration and PAC files to default values. Certificates and rules are preserved.",
	RunE: func(cmd *cobra.Command, args []string) error {
		appDir, err := config.EnsureConfig(true)
		if err != nil {
			return fmt.Errorf("reset failed: %w", err)
		}
		fmt.Printf("Configuration reset to defaults: %s\n", appDir)
		return nil
	},
}

func printConfig(cfg *config.Config) {
	c := color.If
	fmt.Printf("%s=== Configuration ===%s\n", c(color.Bold)+c(color.Cyan), c(color.Reset))

	data, err := toml.Marshal(cfg)
	if err != nil {
		fmt.Printf("(marshal error: %v)\n", err)
		return
	}

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			fmt.Println()
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			fmt.Printf("%s%s%s\n", c(color.Yellow), line, c(color.Reset))
		} else if idx := strings.Index(line, "="); idx > 0 {
			fmt.Printf("%s%s%s=%s%s%s\n", c(color.Green), line[:idx], c(color.Reset), c(color.Cyan), line[idx+1:], c(color.Reset))
		} else {
			fmt.Println(line)
		}
	}
}

func init() {
	configCmd.AddCommand(configShowCmd, configPathCmd, configResetCmd)
}
