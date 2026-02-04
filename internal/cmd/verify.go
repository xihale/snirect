package cmd

import (
	"crypto/x509"
	"encoding/pem"
	"os"

	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:    "verify-cert [cert-path]",
	Short:  "Verify if a certificate is trusted by the system (internal use)",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		certPath := args[0]
		certData, err := os.ReadFile(certPath)
		if err != nil {
			os.Exit(1)
		}

		roots, err := x509.SystemCertPool()
		if err != nil {
			os.Exit(1)
		}

		block, _ := pem.Decode(certData)
		if block == nil {
			os.Exit(1)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			os.Exit(1)
		}

		opts := x509.VerifyOptions{
			Roots: roots,
		}

		if _, err := cert.Verify(opts); err == nil {
			os.Exit(0) // Success
		}
		os.Exit(1) // Failed
	},
}

func init() {
	RootCmd.AddCommand(verifyCmd)
}
