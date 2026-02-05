package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"snirect/internal/ca"
	"snirect/internal/config"
	"snirect/internal/logger"
	"snirect/internal/proxy"
	"snirect/internal/sysproxy"

	"github.com/spf13/cobra"
)

func runProxy(cmd *cobra.Command) error {
	// 1. Ensure config and get app data dir
	appDir, err := config.EnsureConfig(false)
	if err != nil {
		return fmt.Errorf("failed to initialize configuration: %w", err)
	}

	configPath := filepath.Join(appDir, "config.toml")
	rulesPath := filepath.Join(appDir, "rules.toml")
	certDir := filepath.Join(appDir, "certs")

	// Ensure restricted permissions
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return fmt.Errorf("failed to create secure cert dir: %w", err)
	}

	// 2. Load Config
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		logger.Warn("Warning: Failed to load config.toml: %v. Using defaults.", err)
		if cfg == nil {
			return fmt.Errorf("critical error: configuration is invalid and defaults could not be loaded")
		}
	}

	isAutoLaunch := sysproxy.IsLaunchedBySystemOrGUI()
	if isAutoLaunch {
		sysproxy.DisableColor()
		logger.SetColorEnabled(false)
		sysproxy.HideConsole()
	}

	shouldSetProxy := cfg.SetProxy
	if cmd.Flags().Changed("set-proxy") {
		val, _ := cmd.Flags().GetBool("set-proxy")
		shouldSetProxy = val
	} else if isAutoLaunch {
		shouldSetProxy = true
	}

	// Init Logger
	logger.SetLevel(cfg.Log.Level)
	if cfg.Log.File != "" {
		logPath := cfg.Log.File
		if !filepath.IsAbs(logPath) {
			logPath = filepath.Join(appDir, logPath)
		}
		if err := logger.SetOutput(logPath); err != nil {
			fmt.Printf("Failed to set log file: %v\n", err)
		} else {
			logger.Info("日志文件路径: %s", logPath)
		}
	}

	// 3. Init CA
	caCertPath := filepath.Join(certDir, "root.crt")
	caKeyPath := filepath.Join(certDir, "root.key")

	certMgr, err := ca.NewCertManager(caCertPath, caKeyPath)
	if err != nil {
		return fmt.Errorf("failed to initialize CA: %w", err)
	}
	defer certMgr.Close()

	logger.Info("正在启动 Snirect...")
	logger.Info("配置文件加载自: %s", appDir)

	// Handle CA certificate installation based on importca setting
	switch cfg.ImportCA {
	case "never":
		logger.Info("CA 自动安装已禁用 (importca = never)")
	case "always":
		logger.Info("正在强制重新安装 CA 证书 (importca = always)...")
		installed, err := sysproxy.ForceInstallCert(caCertPath)
		if err != nil {
			logger.Warn("Failed to install root CA: %v", err)
			logger.Warn("You may need to install the certificate manually: %s", caCertPath)
		} else if installed {
			logger.Info("Root CA 重新安装成功。请重启浏览器以生效。")
		}
	case "auto", "":
		logger.Info("正在检查根 CA 是否已安装 (importca = auto)...")
		installed, err := sysproxy.InstallCert(caCertPath)
		if err != nil {
			logger.Warn("安装根 CA 失败: %v", err)
			logger.Warn("你可能需要手动安装证书: %s", caCertPath)
		} else if installed {
			logger.Info("Root CA 安装成功。请重启浏览器以生效。")
		}
	default:
		logger.Warn("无效的 importca 值: %q。预期为: auto, always, 或 never。按 auto 处理。", cfg.ImportCA)
		logger.Info("正在检查根 CA 是否已安装...")
		installed, err := sysproxy.InstallCert(caCertPath)
		if err != nil {
			logger.Warn("安装根 CA 失败: %v", err)
			logger.Warn("你可能需要手动安装证书: %s", caCertPath)
		} else if installed {
			logger.Info("Root CA 安装成功。请重启浏览器以生效。")
		}
	}

	rules, err := config.LoadRules(rulesPath)
	if err != nil {
		logger.Warn("Failed to load rules.toml: %v. Rules empty.", err)
		rules = &config.Rules{}
	}

	// 4. Init Proxy
	srv := proxy.NewProxyServer(cfg, rules, certMgr)

	// 5. Start
	serverErr := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil {
			serverErr <- err
		}
	}()

	// 6. Set System Proxy
	if shouldSetProxy {
		time.Sleep(100 * time.Millisecond)
		pacURL := fmt.Sprintf("http://127.0.0.1:%d/pac/?t=%d", cfg.Server.Port, time.Now().Unix())
		sysproxy.SetPAC(pacURL)
		defer sysproxy.ClearPAC()
	}

	// Print Usage Info
	printUsageInfo(cfg.Server.Port)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	select {
	case err := <-serverErr:
		return fmt.Errorf("server failed: %w", err)
	case <-c:
		logger.Info("正在关机...")
	}

	return nil
}

func printUsageInfo(port int) {
	cyan := "\033[36m"
	yellow := "\033[33m"
	green := "\033[32m"
	bold := "\033[1m"
	reset := "\033[0m"

	fmt.Printf("\n%s%s------------------------------------------------------%s\n", bold, cyan, reset)
	fmt.Printf(" %sSnirect%s 正在运行，端口: %s%d%s\n", bold, reset, green, port, reset)
	fmt.Printf("%s------------------------------------------------------%s\n", cyan, reset)
	fmt.Printf(" %s快速设置:%s\n", yellow, reset)
	fmt.Printf("   - 启用系统代理:    %ssnirect set-proxy%s\n", green, reset)
	fmt.Printf("   - 当前终端代理:    %seval $(snirect proxy-env)%s\n", green, reset)
	fmt.Printf("%s------------------------------------------------------%s\n\n", cyan, reset)
}
