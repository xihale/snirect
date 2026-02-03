package cmd

import (
	"fmt"
	"snirect/internal/sysproxy"
	"sort"

	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Check detected system environment",
	Long: `Display information about your system and available tools.
Useful for troubleshooting installation issues.

Checks for:
  - Linux: Desktop Environment (GNOME/KDE), proxy tools, certificate tools
  - macOS: Available system utilities (security, networksetup)
  - Windows: Available system utilities (certutil, reg)`,
	Example: `  snirect env              # Show environment info`,
	Run: func(cmd *cobra.Command, args []string) {
		env := sysproxy.CheckEnv()
		fmt.Println("Detected Environment:")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		keys := make([]string, 0, len(env))
		for k := range env {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			fmt.Printf("  %s: %s\n", k, env[k])
		}
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	},
}

func init() {
	RootCmd.AddCommand(envCmd)
}
