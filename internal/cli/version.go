package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "0.0.0-dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("snirect %s\n", Version)
	},
}
