package cli

import (
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xihale/snirect/internal/color"
	"github.com/xihale/snirect/internal/rules"
)

var (
	rulesDAEPolicy   string
	rulesClashPolicy string
)

var rulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "Inspect and export routing rules",
}

var rulesListCmd = &cobra.Command{
	Use:   "list",
	Short: "Print built-in rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := rules.LoadRules()
		if err != nil {
			return fmt.Errorf("failed to load builtin rules: %w", err)
		}
		printRules(r)
		return nil
	},
}

var rulesMatchCmd = &cobra.Command{
	Use:   "match <host>",
	Short: "Test host against built-in rules",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		host := args[0]
		r, err := rules.LoadRules()
		if err != nil {
			return fmt.Errorf("failed to load builtin rules: %w", err)
		}

		var hostTarget string
		if ip, ok := r.GetHost(host); ok {
			hostTarget = ip
		} else if ips, err := net.LookupHost(host); err == nil && len(ips) > 0 {
			hostTarget = fmt.Sprintf("%s (dns)", ips[0])
		} else {
			hostTarget = "<unresolved>"
		}

		var sniTarget string
		if alt, ok := r.GetAlterHostname(host); ok {
			if alt == "" {
				sniTarget = "<stripped>"
			} else {
				sniTarget = alt
			}
		} else {
			sniTarget = "<unchanged>"
		}

		var certVerifyTarget string
		if pol, ok := r.GetCertVerify(host); ok {
			switch {
			case pol.Strict:
				certVerifyTarget = "strict"
			case len(pol.Allowed) > 0:
				certVerifyTarget = strings.Join(pol.Allowed, ", ")
			case pol.Enabled:
				certVerifyTarget = "true"
			default:
				certVerifyTarget = "false"
			}
		} else {
			certVerifyTarget = "default (true)"
		}

		ignoreExpiry := r.GetIgnoreExpiry(host)

		fmt.Printf("Host:         %s\n", hostTarget)
		fmt.Printf("SNI:          %s\n", sniTarget)
		fmt.Printf("CertVerify:   %s\n", certVerifyTarget)
		fmt.Printf("IgnoreExpiry: %v\n", ignoreExpiry)
		return nil
	},
}

var rulesExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export builtin domain rules for upstream routers",
}

var exportDAECmd = &cobra.Command{
	Use:   "dae",
	Short: "Print builtin domains as a dae routing block",
	RunE:  runDAEExport,
}

var exportClashCmd = &cobra.Command{
	Use:   "clash",
	Short: "Print builtin domains as clash rules",
	RunE:  runClashExport,
}

var rulesDAECmd = &cobra.Command{
	Use:   "dae",
	Short: "Print builtin domains as a dae routing block",
	RunE:  runDAEExport,
}

var rulesClashCmd = &cobra.Command{
	Use:   "clash",
	Short: "Print builtin domains as clash rules",
	RunE:  runClashExport,
}

func runDAEExport(cmd *cobra.Command, args []string) error {
	r, err := rules.LoadRules()
	if err != nil {
		return fmt.Errorf("failed to load builtin rules: %w", err)
	}
	suffixes, exacts := r.ExportDomains()
	fmt.Print(rules.FormatDAE(suffixes, exacts, rulesDAEPolicy))
	return nil
}

func runClashExport(cmd *cobra.Command, args []string) error {
	r, err := rules.LoadRules()
	if err != nil {
		return fmt.Errorf("failed to load builtin rules: %w", err)
	}
	suffixes, exacts := r.ExportDomains()
	fmt.Print(rules.FormatClash(suffixes, exacts, rulesClashPolicy))
	return nil
}

func printRules(r *rules.Rules) {
	c := color.If
	fmt.Printf("\n%s=== Built-in Rules ===%s\n", c(color.Bold)+c(color.Cyan), c(color.Reset))

	printMap := func(title string, m map[string]string) {
		if len(m) == 0 {
			return
		}
		fmt.Printf("%s[%s]%s\n", c(color.Yellow), title, c(color.Reset))
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := m[k]
			if v == "" {
				v = "None"
			}
			fmt.Printf("  %s%s%s -> %s%s%s\n", c(color.Green), k, c(color.Reset), c(color.Cyan), v, c(color.Reset))
		}
		fmt.Println()
	}

	printMap("alter_hostname", r.AlterHostname)
	printMap("hosts", r.Hosts)

	if len(r.CertVerify) > 0 {
		fmt.Printf("%s[cert_verify]%s\n", c(color.Yellow), c(color.Reset))
		keys := make([]string, 0, len(r.CertVerify))
		for k := range r.CertVerify {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("  %s%s%s -> %s%v%s\n", c(color.Green), k, c(color.Reset), c(color.Cyan), r.CertVerify[k], c(color.Reset))
		}
	}
}

func init() {
	exportDAECmd.Flags().StringVar(&rulesDAEPolicy, "policy", "snirect", "dae group the rules point to")
	rulesDAECmd.Flags().StringVar(&rulesDAEPolicy, "policy", "snirect", "dae group the rules point to")
	exportClashCmd.Flags().StringVar(&rulesClashPolicy, "policy", "snirect", "clash policy the rules point to")
	rulesClashCmd.Flags().StringVar(&rulesClashPolicy, "policy", "snirect", "clash policy the rules point to")

	rulesDAECmd.Hidden = true
	rulesClashCmd.Hidden = true

	rulesExportCmd.AddCommand(exportDAECmd, exportClashCmd)
	rulesCmd.AddCommand(rulesListCmd, rulesMatchCmd, rulesExportCmd, rulesDAECmd, rulesClashCmd)
}
