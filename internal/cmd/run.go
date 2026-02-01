package cmd

import (
	"fmt"

	"os"

	"os/signal"

	"path/filepath"

	"snirect/internal/ca"

	"snirect/internal/config"

	"snirect/internal/logger"

	"snirect/internal/proxy"

	"snirect/internal/sysproxy"

	"time"

	"github.com/spf13/cobra"
)

func runProxy(cmd *cobra.Command) {

	// 1. Ensure config and get app data dir

	appDir, err := config.EnsureConfig(false)

	if err != nil {

		fmt.Fprintf(os.Stderr, "Failed to initialize configuration: %v\n", err)

		os.Exit(1)

	}

	configPath := filepath.Join(appDir, "config.toml")

	rulesPath := filepath.Join(appDir, "rules.toml")

	certDir := filepath.Join(appDir, "certs")

	// Ensure restricted permissions

	if err := os.MkdirAll(certDir, 0700); err != nil {

		fmt.Fprintf(os.Stderr, "Failed to create secure cert dir: %v\n", err)

		os.Exit(1)

	}

	// 2. Load Config

	cfg, err := config.LoadConfig(configPath)

	if err != nil {

		fmt.Printf("Warning: Failed to load config.toml: %v. Using defaults.\n", err)

	}

	// Override setproxy from config if flag is explicitly passed

	shouldSetProxy := cfg.SetProxy

	if cmd.Flags().Changed("set-proxy") {

		val, _ := cmd.Flags().GetBool("set-proxy")

		shouldSetProxy = val

	}

	// Init Logger
	logger.SetLevel(cfg.Log.Level)
	if cfg.Log.File != "" {
		if err := logger.SetOutput(cfg.Log.File); err != nil {
			fmt.Printf("Failed to set log file: %v\n", err)
		}
	}

	// 3. Init CA
	caCertPath := filepath.Join(certDir, "root.crt")
	caKeyPath := filepath.Join(certDir, "root.key")

	certMgr, err := ca.NewCertManager(caCertPath, caKeyPath)
	if err != nil {
		logger.Fatal("Failed to initialize CA: %v", err)
	}

	logger.Info("Starting Snirect...")
	logger.Info("Config loaded from: %s", appDir)
	logger.Info("CA initialized. Root cert: %s", caCertPath)

	rules, err := config.LoadRules(rulesPath)
	if err != nil {
		logger.Warn("Failed to load rules.toml: %v. Rules empty.", err)
		rules = &config.Rules{}
	}

	// 4. Init Proxy
	srv := proxy.NewProxyServer(cfg, rules, certMgr)

	// 5. Start
	go func() {
		if err := srv.Start(); err != nil {
			logger.Fatal("Server failed: %v", err)
		}
	}()

	// 6. Set System Proxy
	if shouldSetProxy {
		pacURL := fmt.Sprintf("http://127.0.0.1:%d/pac/?t=%d", cfg.Server.Port, time.Now().Unix())
		sysproxy.SetPAC(pacURL)
		defer sysproxy.ClearPAC()
	}

	// Print Usage Info
	printUsageInfo(cfg.Server.Port)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	logger.Info("Shutting down...")
}

func printUsageInfo(port int) {
	cyan := "\033[36m"
	yellow := "\033[33m"
	green := "\033[32m"
	bold := "\033[1m"
	reset := "\033[0m"

	fmt.Printf("\n%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", bold, cyan, reset)
	fmt.Printf(" %sSnirect%s is running on port %s%d%s\n", bold, reset, green, port, reset)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", cyan, reset)
	fmt.Printf(" %sQuick Setup:%s\n", yellow, reset)
	fmt.Printf("   - Use system proxy:  %ssnirect set-proxy%s\n", green, reset)
	fmt.Printf("   - Current terminal:  %seval $(snirect proxy-env)%s\n", green, reset)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n", cyan, reset)
}
