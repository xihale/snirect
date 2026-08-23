package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	setProxy  bool
	logLevel  string
	pprof     bool
	pprofAddr string
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
				fmt.Printf("  Set proxy:  snirect proxy set\n")
				fmt.Printf("  Shell env:  eval $(snirect proxy env)\n\n")
			},
		})
	},
}

func init() {
	rootCmd.Flags().BoolVarP(&setProxy, "set-proxy", "s", false, "Set system proxy on start")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "v", "", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVar(&pprof, "pprof", false, "Enable pprof profiling")
	rootCmd.PersistentFlags().StringVar(&pprofAddr, "pprof-addr", "127.0.0.1:6060", "pprof listen address")

	certCmd.AddCommand(certInstallCmd, certRemoveCmd, certStatusCmd)
	certCmd.AddCommand(firefoxCertCmd)
	firefoxCertCmd.AddCommand(firefoxInstallCmd, firefoxRemoveCmd, firefoxStatusCmd)

	proxyCmd.AddCommand(proxySetCmd, proxyUnsetCmd, proxyEnvCmd)

	configCmd.AddCommand(configResetCmd)

	rootCmd.AddCommand(
		installCmd,
		uninstallCmd,
		certCmd,
		proxyCmd,
		configCmd,
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
