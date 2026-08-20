package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/xihale/snirect/app"
	"github.com/xihale/snirect/color"
	"github.com/xihale/snirect/config"
	"github.com/xihale/snirect/rules"
	"github.com/xihale/snirect/sysproxy"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show comprehensive status",
	Run: func(cmd *cobra.Command, args []string) {
		if showConfig {
			printConfigAndRules()
			return
		}
		printStatus()
	},
}

var showConfig bool

func init() {
	statusCmd.Flags().BoolVar(&showConfig, "show-config", false, "Print loaded config and built-in rules")
}

// statusView is the normalized shape gatherStatusFromFile produces and
// renderStatus consumes. There is exactly one renderer for one data source.
type statusView struct {
	appDir       string
	configPort   int    // 0 = unknown
	configErr    string // non-empty = config failed to load
	portOccupied bool   // a process is listening on configPort
	portOwner    string // process name owning configPort (best-effort)
	portErr      string // non-empty when the port probe itself failed
	certExists   bool
	certTrust    string // "installed" | "not installed" | "unknown"
	certErr      string
	systemProxy  bool
	svcRunning   bool
	svcInstalled bool
	svcDetail    string
	logFile      string
}

// printStatus shows the snirect status. There is no longer a live daemon HTTP
// surface to query, so status is read entirely from disk plus a probe of the
// configured proxy port to infer whether the proxy is up.
func printStatus() {
	v := gatherStatus()
	c := color.If
	fmt.Printf("\n%sSnirect Status%s\n", c(color.Bold)+c(color.Cyan), c(color.Reset))
	fmt.Println("────────────────────────────────────────────")
	renderStatus(v)
}

// gatherStatus reads config/cert/proxy/service from disk and platform tools,
// then probes the configured proxy port to infer runtime proxy health.
func gatherStatus() statusView {
	appDir, err := config.GetAppDataDir()
	if err != nil {
		appDir = "unknown"
	}
	cfg, cfgErr := config.LoadConfig(filepath.Join(appDir, "config.toml"))

	v := statusView{appDir: appDir}
	if cfgErr != nil {
		v.configErr = cfgErr.Error()
	} else {
		v.configPort = cfg.Server.Port
		v.logFile = cfg.Log.File
	}

	certPath := filepath.Join(appDir, "certs", "root.crt")
	if _, err := os.Stat(certPath); err == nil {
		v.certExists = true
		installed, err := sysproxy.CheckCertStatus(certPath)
		switch {
		case err != nil:
			v.certErr = err.Error()
			v.certTrust = "unknown"
		default:
			v.certTrust = trustLabel(installed)
		}
	}

	v.systemProxy = sysproxy.IsSystemProxySet()
	v.svcInstalled, v.svcRunning, v.svcDetail = app.ServiceStatus()

	// Port probe — the replacement for the deleted daemon control API. We only
	// have a real answer for the HTTP-proxy listener; TUN mode (no TCP port)
	// leaves this inconclusive.
	p := probeProxyPort(v.configPort)
	v.portOccupied = p.occupied
	v.portOwner = p.owner
	v.portErr = p.err
	return v
}

func trustLabel(installed bool) string {
	if installed {
		return "installed"
	}
	return "not installed"
}

// renderStatus is the single printer for a statusView.
func renderStatus(v statusView) {
	c := color.If
	header := func(title string) { fmt.Printf("\n%s%s%s\n", c(color.Bold), title, c(color.Reset)) }

	header("Config")
	fmt.Printf("  Directory: %s\n", v.appDir)
	switch {
	case v.configErr != "":
		fmt.Printf("  Status:    %sload error%s (%s)\n", c(color.Yellow), c(color.Reset), v.configErr)
	default:
		fmt.Printf("  Status:    %sloaded%s\n", c(color.Green), c(color.Reset))
	}
	if v.configPort != 0 {
		fmt.Printf("  Port:      %d\n", v.configPort)
	}

	header("Daemon")
	switch {
	case v.portErr != "":
		// Probe failed (no port configured, or lsof/netstat unavailable).
		fmt.Printf("  Port:      %s%s%s\n", c(color.Gray), v.portErr, c(color.Reset))
	case !v.portOccupied:
		fmt.Printf("  Port:      %s%d not listening%s  (start with: snirect)\n", c(color.Gray), v.configPort, c(color.Reset))
	case looksLikeSnirect(v.portOwner):
		fmt.Printf("  Port:      %s%d in use by snirect%s  (proxy running)\n", c(color.Green), v.configPort, c(color.Reset))
	default:
		// Someone else is squatting the configured port — almost certainly a
		// misconfiguration, surface it loudly.
		fmt.Printf("  Port:      %s%d in use by `%s`%s  — expected snirect!\n", c(color.Yellow), v.configPort, v.portOwner, c(color.Reset))
	}

	header("Certificate")
	if !v.certExists {
		fmt.Printf("  File:      %snot found%s  → run snirect to generate\n", c(color.Red), c(color.Reset))
	} else {
		fmt.Printf("  File:      %sexists%s\n", c(color.Green), c(color.Reset))
		switch {
		case v.certErr != "":
			fmt.Printf("  Trust:     %sunknown%s (%s)\n", c(color.Yellow), c(color.Reset), v.certErr)
		case v.certTrust == "installed":
			fmt.Printf("  Trust:     %sinstalled%s\n", c(color.Green), c(color.Reset))
		default:
			fmt.Printf("  Trust:     %snot installed%s  → snirect cert install\n", c(color.Red), c(color.Reset))
		}
	}

	header("Proxy")
	if v.systemProxy {
		fmt.Printf("  System:    %senabled%s\n", c(color.Green), c(color.Reset))
	} else {
		fmt.Printf("  System:    %snot set%s  → snirect proxy set\n", c(color.Red), c(color.Reset))
	}

	header("Service")
	switch {
	case v.svcRunning:
		fmt.Printf("  Status:    %srunning%s\n", c(color.Green), c(color.Reset))
	case v.svcInstalled:
		fmt.Printf("  Status:    %sinstalled%s (%s)\n", c(color.Yellow), c(color.Reset), v.svcDetail)
	case v.svcDetail != "":
		fmt.Printf("  Status:    %s%s%s\n", c(color.Yellow), v.svcDetail, c(color.Reset))
	default:
		fmt.Printf("  Status:    %snot installed%s\n", c(color.Red), c(color.Reset))
	}

	header("Quick Commands")
	fmt.Println("  Start:       snirect -s")
	fmt.Println("  Install CA:  snirect cert install")
	fmt.Println("  Set proxy:   snirect proxy set")
	fmt.Printf("  Logs:        %s\n", logHint(v.logFile))
	fmt.Println("────────────────────────────────────────────")
}

func logHint(logFile string) string {
	if logFile == "" {
		return "snirect (console output)"
	}
	abs, _ := filepath.Abs(logFile)
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(`Get-Content -Wait "%s"`, abs)
	}
	return fmt.Sprintf("tail -f %s", abs)
}

// --- config & rules dump (`snirect status --show-config`) -----------------

// printConfigAndRules prints the effective config (as TOML) and the built-in
// routing rules, with ANSI coloring. Reached via `snirect status --show-config`.
func printConfigAndRules() {
	cfg, _, err := loadAppConfig()
	if err != nil {
		fmt.Printf("config load error: %v\n", err)
		return
	}

	r, err := rules.LoadRules()
	if err != nil {
		fmt.Printf("rules load error: %v\n", err)
		return
	}

	printConfig(cfg)
	printRules(r)
}

func printConfig(cfg *config.Config) {
	c := color.If
	fmt.Printf("%s=== Configuration ===%s\n", c(color.Bold)+c(color.Cyan), c(color.Reset))

	data, err := toml.Marshal(cfg)
	if err != nil {
		fmt.Printf("(marshal error: %v)\n", err)
		return
	}

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			fmt.Println()
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			fmt.Printf("%s%s%s\n", c(color.Yellow), line, c(color.Reset))
		} else if idx := strings.Index(line, "="); idx > 0 {
			fmt.Printf("%s%s%s=%s%s%s\n", c(color.Green), line[:idx], c(color.Reset), c(color.Cyan), line[idx+1:], c(color.Reset))
		} else {
			fmt.Println(line)
		}
	}
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
