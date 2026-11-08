package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	setProxy    bool
	portFlag    int
	configFile  string
	logLevel    string
	verboseFlag bool
)

var rootCmd = &cobra.Command{
	Use:   "snirect",
	Short: "Cross-platform SNI proxy for bypassing censorship",
	Long: `Snirect is a transparent HTTP/HTTPS proxy that modifies SNI
(Server Name Indication) to bypass SNI-based censorship.

Supports: Linux, macOS, Windows`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunProxy(cmd, RunOptions{
			OnReady: func(port int) {
				fmt.Printf("\nSnirect running on port %d\n", port)
				fmt.Printf("  Enable proxy: snirect proxy on\n")
				fmt.Printf("  Shell env:    eval $(snirect proxy env)\n\n")
			},
		})
	},
}

var runCmd = &cobra.Command{
	Use:           "run",
	Aliases:       []string{"serve"},
	Short:         "Run proxy server in the foreground",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunProxy(cmd, RunOptions{
			OnReady: func(port int) {
				fmt.Printf("\nSnirect running on port %d\n", port)
				fmt.Printf("  Enable proxy: snirect proxy on\n")
				fmt.Printf("  Shell env:    eval $(snirect proxy env)\n\n")
			},
		})
	},
}

func init() {
	rootCmd.Version = Version

	rootCmd.PersistentFlags().BoolVarP(&setProxy, "set-proxy", "s", false, "Set system proxy on start")
	rootCmd.PersistentFlags().IntVarP(&portFlag, "port", "p", 0, "Override listen port (0 = use config)")
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Custom config.toml path")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "Verbose output (shorthand for --log-level debug)")

	rootCmd.AddCommand(
		runCmd,
		serviceCmd,
		certCmd,
		proxyCmd,
		configCmd,
		rulesCmd,
		statusCmd,
		versionCmd,
		updateCmd,
		completionCmd,
	)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
