package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "0.0.0-dev"

var versionCmd = &cobra.Command{
	Use:     "version",
	Aliases: []string{"v"},
	Short:   "Show version and module information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Snirect version: %s\n", Version)
		fmt.Println("Module Status:")
		for _, m := range getModuleStatus() {
			status := "[-]"
			if m.Enabled {
				status = "[+]"
			}
			fmt.Printf("  %s %s\n", status, m.Name)
		}
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
