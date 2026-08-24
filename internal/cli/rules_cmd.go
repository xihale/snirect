package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xihale/snirect/internal/rules"
)

var (
	rulesDAEPolicy   string
	rulesClashPolicy string
)

var rulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "Export builtin domain rules for upstream routers",
	Long: `Export the builtin host rules as domain rules for an upstream
router (dae, clash), so traffic for the domains snirect knows
about can be handed to snirect.

Only domain rules are printed — point them at the policy/group
of your snirect outbound with --policy.`,
}

var rulesDAECmd = &cobra.Command{
	Use:   "dae",
	Short: "Print builtin domains as a dae routing block",
	Long: `Print the builtin host rules as a dae domain() block for the
routing section, pointing at --policy (default "snirect"):

  domain(
      suffix: github.com
      full: docker.io
  ) -> snirect`,
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := rules.LoadRules()
		if err != nil {
			return fmt.Errorf("failed to load builtin rules: %w", err)
		}
		suffixes, exacts := r.ExportDomains()
		fmt.Print(rules.FormatDAE(suffixes, exacts, rulesDAEPolicy))
		return nil
	},
}

var rulesClashCmd = &cobra.Command{
	Use:   "clash",
	Short: "Print builtin domains as clash rules",
	Long: `Print the builtin host rules as clash/mihomo rule lines for a
rules list, pointing at --policy (default "snirect"):

  - DOMAIN-SUFFIX,github.com,snirect
  - DOMAIN,docker.io,snirect`,
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := rules.LoadRules()
		if err != nil {
			return fmt.Errorf("failed to load builtin rules: %w", err)
		}
		suffixes, exacts := r.ExportDomains()
		fmt.Print(rules.FormatClash(suffixes, exacts, rulesClashPolicy))
		return nil
	},
}
