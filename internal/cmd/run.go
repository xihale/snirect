package cmd

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"

	"snirect/internal/cert"
	"snirect/internal/config"
	"snirect/internal/container"
	"snirect/internal/logger"
	"snirect/internal/sysproxy"
	"snirect/internal/update"

	"github.com/spf13/cobra"
)

func runProxy(cmd *cobra.Command) error {
	appDir, err := config.EnsureConfig(false)
	if err != nil {
		return fmt.Errorf("failed to initialize configuration: %w", err)
	}

	configPath := filepath.Join(appDir, "config.toml")
	rulesPath := filepath.Join(appDir, "rules.toml")
	certDir := filepath.Join(appDir, "certs")

	if err := os.MkdirAll(certDir, 0700); err != nil {
		return fmt.Errorf("failed to create secure cert dir: %w", err)
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		logger.Warn("Warning: Failed to load config.toml: %v. Using defaults.", err)
		if cfg == nil {
			return fmt.Errorf("critical error: configuration is invalid and defaults could not be loaded")
		}
	}

	if update.HasPendingUpdate(appDir) {
		logger.Info("Pending update detected. Performing self-update...")
		if err := update.PerformSelfUpdate(appDir); err != nil {
			logger.Error("Self-update failed: %v", err)
		} else {
			return nil
		}
	}

	if cfg.Update.AutoCheckRules || cfg.Update.AutoUpdateRules {
		shouldCheck, err := config.ShouldCheckRules(appDir, time.Duration(cfg.Update.RulesCheckIntervalHours)*time.Hour)
		if err != nil {
			logger.Warn("Failed to check rules timestamp: %v", err)
		} else if shouldCheck && cfg.Update.AutoUpdateRules {
			logger.Info("Checking for rules updates...")
			mgr := update.NewManager(cfg, &config.Rules{})
			if err := mgr.FetchRules(appDir); err != nil {
				logger.Warn("Auto rules update failed: %v", err)
			} else {
				logger.Info("Rules auto-updated successfully")
			}
		}
	}

	if cfg.Update.AutoCheckUpdate || cfg.Update.AutoUpdate {
		shouldCheck, err := config.ShouldCheckUpdate(appDir, time.Duration(cfg.Update.CheckIntervalHours)*time.Hour)
		if err != nil {
			logger.Warn("Failed to check update timestamp: %v", err)
		} else if shouldCheck {
			logger.Info("Checking for program updates...")
			mgr := update.NewManager(cfg, &config.Rules{})
			hasUpdate, latestVersion, err := mgr.CheckForUpdate(Version)
			mgr.UpdateCheckTimestamp(appDir)
			if err != nil {
				logger.Warn("Auto update check failed: %v", err)
			} else if hasUpdate && cfg.Update.AutoUpdate {
				logger.Info("Update available: %s -> %s", Version, latestVersion)
				assetName := fmt.Sprintf("snirect-%s-%s", runtime.GOOS, runtime.GOARCH)
				if runtime.GOOS == "windows" {
					assetName += ".exe"
				}
				downloadPath := filepath.Join(mgr.GetTempDownloadDir(appDir), assetName)
				downloadURL := fmt.Sprintf("https://github.com/xihale/snirect/releases/download/%s/%s", latestVersion, assetName)
				logger.Info("Downloading update from: %s", downloadURL)
				if err := mgr.DownloadFile(downloadPath, downloadURL); err != nil {
					logger.Warn("Auto update download failed: %v", err)
				} else {
					if runtime.GOOS != "windows" {
						os.Chmod(downloadPath, 0755)
					}
					if err := mgr.MarkForSelfUpdate(appDir, latestVersion); err != nil {
						logger.Warn("Failed to mark update: %v", err)
					} else {
						logger.Info("Update prepared. Restart to apply.")
					}
				}
			} else if hasUpdate {
				logger.Info("Update available: %s (auto-update disabled)", latestVersion)
			}
		}
	}

	isAutoLaunch := sysproxy.IsLaunchedBySystemOrGUI()
	if isAutoLaunch {
		sysproxy.DisableColor()
		logger.SetColorEnabled(false)
		if sysproxy.IsSilentLaunch() {
			sysproxy.HideConsole()
		}
	}

	shouldSetProxy := cfg.SetProxy
	if cmd.Flags().Changed("set-proxy") {
		val, _ := cmd.Flags().GetBool("set-proxy")
		shouldSetProxy = val
	}

	finalLogLevel := cfg.Log.Level
	if logLevel != "" {
		finalLogLevel = logLevel
	}
	logger.SetLevel(finalLogLevel)
	if cfg.Log.File != "" {
		logPath := cfg.Log.File
		if !filepath.IsAbs(logPath) {
			logPath = filepath.Join(appDir, logPath)
		}
		if err := logger.SetOutput(logPath); err != nil {
			fmt.Printf("Failed to set log file: %v\n", err)
		} else {
			logger.Info("Log file: %s", logPath)
		}
	}

	// Start pprof profiling server if enabled
	if pprof {
		go func() {
			logger.Info("pprof server listening on %s (endpoints: /debug/pprof/, /debug/pprof/heap, /debug/pprof/goroutine, etc.)", pprofAddr)
			if err := http.ListenAndServe(pprofAddr, nil); err != nil {
				logger.Error("pprof server failed: %v", err)
			}
		}()
	}

	caCertPath := filepath.Join(certDir, "root.crt")
	caKeyPath := filepath.Join(certDir, "root.key")

	logger.Info("Starting Snirect...")
	logger.Info("Config directory: %s", appDir)

	// Load rules first (needed for container)
	rules, err := config.LoadRules(rulesPath)
	if err != nil {
		logger.Warn("Failed to load rules.toml: %v. Rules empty.", err)
		rules = &config.Rules{}
	}

	// Create container for dependency injection
	cnt := container.New(cfg, rules)

	// Initialize certificate manager with actual paths
	certMgr, err := cert.NewCertificateManager(caCertPath, caKeyPath)
	if err != nil {
		return fmt.Errorf("failed to initialize CA: %w", err)
	}
	cnt.SetCertManager(certMgr)
	defer cnt.Close()

	switch cfg.CAInstall {
	case "never":
		logger.Info("CA auto-install disabled")
	case "always":
		logger.Info("Force re-installing root CA...")
		installed, err := sysproxy.ForceInstallCert(caCertPath)
		if err != nil {
			logger.Warn("Failed to install root CA: %v", err)
			logger.Warn("You may need to install manually: %s", caCertPath)
		} else if installed {
			logger.Info("Root CA reinstalled. Restart browser to apply.")
		}
	case "auto", "":
		logger.Info("Checking root CA installation...")
		installed, err := sysproxy.InstallCert(caCertPath)
		if err != nil {
			logger.Warn("Failed to install root CA: %v", err)
			logger.Warn("You may need to install manually: %s", caCertPath)
		} else if installed {
			logger.Info("Root CA installed. Restart browser to apply.")
		}
	default:
		logger.Warn("Invalid ca_install: %q. Expected: auto, always, never.", cfg.CAInstall)
		logger.Info("Checking root CA...")
		installed, err := sysproxy.InstallCert(caCertPath)
		if err != nil {
			logger.Warn("Failed to install root CA: %v", err)
			logger.Warn("You may need to install manually: %s", caCertPath)
		} else if installed {
			logger.Info("Root CA installed. Restart browser to apply.")
		}
	}

	// Create proxy server via container (uses injected certMgr and resolver)
	srv := cnt.GetProxyServer()

	serverErr := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil {
			serverErr <- err
		}
	}()

	if shouldSetProxy {
		time.Sleep(100 * time.Millisecond)
		pacURL := fmt.Sprintf("http://127.0.0.1:%d/pac/?t=%d", cfg.Server.Port, time.Now().Unix())
		sysproxy.SetPAC(pacURL)
		defer sysproxy.ClearPAC()
	}

	printUsageInfo(cfg.Server.Port)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	select {
	case err := <-serverErr:
		return fmt.Errorf("server failed: %w", err)
	case <-c:
		logger.Info("Shutting down...")
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
