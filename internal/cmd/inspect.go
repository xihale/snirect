package cmd

import (
	"fmt"
	"path/filepath"
	"snirect/internal/config"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "输出当前已加载的配置和规则 (合并默认值后)",
	Long: `该命令显示 Snirect 当前使用的完整配置和分流规则。
它会显示合并了硬编码默认值、嵌入的默认配置文件以及用户自定义配置文件后的最终状态。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appDir, err := config.GetAppDataDir()
		if err != nil {
			return err
		}

		configPath := filepath.Join(appDir, "config.toml")
		rulesPath := filepath.Join(appDir, "rules.toml")

		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		rules, err := config.LoadRules(rulesPath)
		if err != nil {
			return fmt.Errorf("failed to load rules: %w", err)
		}

		bold := "\033[1m"
		cyan := "\033[36m"
		reset := "\033[0m"
		green := "\033[32m"
		yellow := "\033[33m"

		fmt.Printf("%s%s=== 当前配置 (config.toml) ===%s\n", bold, cyan, reset)
		cfgData, err := toml.Marshal(cfg)
		if err != nil {
			return err
		}

		for _, line := range strings.Split(string(cfgData), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				fmt.Println()
				continue
			}
			if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
				fmt.Printf("%s%s%s\n", yellow, line, reset)
			} else if idx := strings.Index(line, "="); idx != -1 {
				key := line[:idx]
				val := line[idx+1:]
				fmt.Printf("%s%s%s=%s%s%s\n", green, key, reset, cyan, val, reset)
			} else {
				fmt.Println(line)
			}
		}

		fmt.Printf("\n%s%s=== 当前规则 (rules.toml) ===%s\n", bold, cyan, reset)

		printMap := func(title string, m map[string]string) {
			if len(m) == 0 {
				return
			}
			fmt.Printf("%s[%s]%s\n", yellow, title, reset)
			// Sort keys for consistent output
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
				fmt.Printf("  %s%s%s -> %s%s%s\n", green, k, reset, cyan, v, reset)
			}
			fmt.Println()
		}

		printMap("alter_hostname", rules.AlterHostname)
		printMap("hosts", rules.Hosts)

		if len(rules.CertVerify) > 0 {
			fmt.Printf("%s[cert_verify]%s\n", yellow, reset)
			keys := make([]string, 0, len(rules.CertVerify))
			for k := range rules.CertVerify {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				v := rules.CertVerify[k]
				fmt.Printf("  %s%s%s -> %s%v%s\n", green, k, reset, cyan, v, reset)
			}
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(inspectCmd)
}
