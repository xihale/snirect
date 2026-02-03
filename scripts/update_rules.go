package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	CealingHostURL     = "https://github.com/SpaceTimee/Cealing-Host/releases/download/1.1.4.41/Cealing-Host.toml"
	CealingHostVersion = "1.1.4.41"
)

type CealingHostConfig struct {
	AlterHostname map[string]string `toml:"alter_hostname"`
	Hosts         map[string]string `toml:"hosts"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Printf("Downloading Cealing-Host %s...\n", CealingHostVersion)
	client := &http.Client{}
	req, err := http.NewRequest("GET", CealingHostURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var cealingHost CealingHostConfig
	if err := toml.Unmarshal(data, &cealingHost); err != nil {
		return fmt.Errorf("failed to parse TOML: %w", err)
	}

	fmt.Printf("Loaded %d alter_hostname rules and %d hosts rules\n",
		len(cealingHost.AlterHostname), len(cealingHost.Hosts))

	rulesPath := filepath.Join("..", "internal", "config", "rules.toml")
	pacPath := filepath.Join("..", "internal", "config", "pac")

	if err := updateRules(rulesPath, data); err != nil {
		return fmt.Errorf("failed to update rules.toml: %w", err)
	}

	if err := updatePAC(pacPath, &cealingHost); err != nil {
		return fmt.Errorf("failed to generate pac: %w", err)
	}

	fmt.Println("✓ Successfully updated rules and PAC!")
	return nil
}

func updateRules(path string, tomlData []byte) error {
	header := `# 这是Snirect的规则文件，编译时已嵌入程序。
# This is the rule file of Snirect, which is embedded in the program at compile time.

[DNS]
# 默认使用 dnschina1.soraharu.com、Yandex DNS 和 Google DNS。
# 使用 Google DNS 节点 (https://dns.google/dns-query) 可以更好地支持 EDNS (ECS) 信息，加快访问速度。
# Default nameserver is set to soraharu, Yandex and Google for better accessibility.
# Google DNS is included for robust EDNS (ECS) support.
nameserver = ["https://dnschina1.soraharu.com/dns-query", "https://77.88.8.8/dns-query", "https://dns.google/dns-query"]

# 用于解析 DNS 服务器域名的引导 DNS (NS Namespace)。
# Bootstrap DNS used to resolve DNS server hostnames.
bootstrap_dns = ["tls://223.5.5.5"]

[cert_verify]
# ⚠️ 安全警告：
# 默认规则中的部分 IP（如 Google 相关的 34.49.133.3）是第三方公益代理服务器。
# 由于这些服务器返回的证书域名与原始域名不匹配，必须关闭校验（false）才能正常使用。
# 这会带来潜在的中间人攻击风险。
# TODO: 未来应寻找并使用真实的 GGC IP，并恢复证书校验。
# 
# ⚠️ SECURITY WARNING:
# Some IPs in default rules (e.g. 34.49.133.3 for Google) are third-party public proxy servers.
# Since these servers return certificates that do not match the original domain, 
# verification must be disabled (false) to function. This poses a potential MITM risk.
# TODO: Use real GGC IPs and restore certificate verification in the future.
"t.me" = false
"*.t.me" = false

`
	if err := os.WriteFile(path, append([]byte(header), tomlData...), 0644); err != nil {
		return err
	}

	fmt.Printf("✓ Updated %s\n", path)
	return nil
}

func updatePAC(path string, cealing *CealingHostConfig) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	pacDomainsStr := generatePACDomains(cealing.AlterHostname, cealing.Hosts)

	newContent := string(content)

	startMarker := "var domains = {"
	endMarker := "};"

	startIdx := strings.Index(newContent, startMarker)
	if startIdx == -1 {
		return fmt.Errorf("could not find domains start marker")
	}
	endIdx := strings.Index(newContent[startIdx:], endMarker)
	if endIdx == -1 {
		return fmt.Errorf("could not find domains end marker")
	}
	endIdx += startIdx

	finalContent := newContent[:startIdx+len(startMarker)] + "\n" + pacDomainsStr + "\n" + newContent[endIdx:]

	if err := os.WriteFile(path, []byte(finalContent), 0644); err != nil {
		return err
	}

	fmt.Printf("✓ Updated %s\n", path)
	return nil
}

func generatePACDomains(alterHostname, hosts map[string]string) string {
	domainSet := make(map[string]bool)

	for key := range alterHostname {
		if d := extractRootDomain(key); d != "" {
			domainSet[d] = true
		}
	}
	for key := range hosts {
		if d := extractRootDomain(key); d != "" {
			domainSet[d] = true
		}
	}

	domains := make([]string, 0, len(domainSet))
	for d := range domainSet {
		domains = append(domains, d)
	}
	sort.Strings(domains)

	var lines []string
	for _, domain := range domains {
		lines = append(lines, fmt.Sprintf("  \"%s\": 1,", domain))
	}

	return strings.Join(lines, "\n")
}

func extractRootDomain(pattern string) string {
	if strings.HasPrefix(pattern, "#") {
		return ""
	}
	domain := strings.TrimPrefix(pattern, "$")
	domain = strings.TrimPrefix(domain, "*")
	domain = strings.TrimPrefix(domain, ".")

	if idx := strings.Index(domain, "^"); idx != -1 {
		domain = domain[:idx]
	}

	parts := strings.Split(domain, ".")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], ".")
	}

	return domain
}
