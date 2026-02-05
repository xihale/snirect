package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile  string
	setProxy bool
	logLevel string
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "snirect",
	Short: "Cross-platform tool to bypass SNI-based censorship",
	Long: `Snirect is a transparent HTTP/HTTPS proxy that modifies SNI (Server Name Indication)
to bypass censorship/blocking based on SNI RST.

Supports: Linux, macOS, and Windows`,
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runProxy(cmd)
	},
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.Flags().BoolVarP(&setProxy, "set-proxy", "s", false, "Set system proxy automatically")
	RootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "v", "", "Set log level (debug, info, warn, error)")
}

func initConfig() {
	// Initialize logger config here if needed, but we do it in runProxy usually
	// For now just basic setup
}

func GetRootCmd() *cobra.Command {
	return RootCmd
}
