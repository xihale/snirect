package cmd

import (
	"fmt"
	"sort"
	"snirect/internal/sysproxy"

	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Check detected system environment",
	Long:  `Checks and lists the detected Operating System, Desktop Environment, and available proxy/certificate management tools.`,
	Run: func(cmd *cobra.Command, args []string) {
		env := sysproxy.CheckEnv()
		fmt.Println("Detected Environment:")
		
		keys := make([]string, 0, len(env))
		for k := range env {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			fmt.Printf("  %s: %s\n", k, env[k])
		}
	},
}

func init() {
	RootCmd.AddCommand(envCmd)
}

