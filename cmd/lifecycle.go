package cmd

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/xihale/snirect/cert"
	"github.com/xihale/snirect/config"
	"github.com/xihale/snirect/dns"
	"github.com/xihale/snirect/logger"
	"github.com/xihale/snirect/proxy"
	"github.com/xihale/snirect/sysproxy"

	"github.com/spf13/cobra"
)

// ProxyEnv holds a bootstrapped proxy ready to run, shared by the bare
// `snirect` entrypoint via RunProxy. The owner calls Cleanup when done.
type ProxyEnv struct {
	Cfg     *config.Config
	AppDir  string
	certMgr *cert.CertificateManager
	// resolver is the shared resolver behind both the proxy and the upstream
	// client; closed in Cleanup.
	resolver *dns.Resolver
	Server   proxyServer
	// pacSet is true if this process programmed the system PAC. Only the main
	// goroutine touches it (set before serving, read in Cleanup after serving
	// ends), so it needs no lock. This fixes the old predicate that gated
	// cleanup on the --set-proxy flag.
	pacSet bool
}

// Cleanup clears the system PAC iff this process set it, then closes the
// resolver and certificate manager.
func (env *ProxyEnv) Cleanup() {
	if env.pacSet {
		sysproxy.ClearPAC()
	}
	if env.resolver != nil {
		env.resolver.Close()
	}
	if env.certMgr != nil {
		env.certMgr.Close()
	}
}

// proxyServer is the subset of *proxy.ProxyServer the lifecycle uses. Extracted
// as an interface so tests can substitute a fake.
type proxyServer interface {
	Listen() error
	Serve() error
	Shutdown(ctx context.Context) error
}

// RunOptions configures RunProxy. Today the only option is OnReady; the type
// is kept so callers can grow it without touching RunProxy's signature.
type RunOptions struct {
	OnReady func(port int)
}

// BootstrapProxy performs the shared startup preparation: load config, build
// the resolver + CA + proxy server. It does NOT start serving. On error any
// partial state is cleaned up.
func BootstrapProxy(cmd *cobra.Command) (*ProxyEnv, error) {
	cfg, appDir, err := loadAppConfig()
	if err != nil {
		return nil, err
	}

	cd := certDir(appDir)
	if err := os.MkdirAll(cd, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create cert dir: %w", err)
	}

	setupLogging(cfg, cmd)
	startPprof()

	r, err := config.LoadRules()
	if err != nil {
		logger.Control().Warn("failed to load rules", "error", err)
	}

	cm, err := cert.NewCertificateManager(certPath(appDir), filepath.Join(cd, "root.key"))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize CA: %w", err)
	}

	installCA(cfg, appDir)

	resolver := dns.NewResolver(cfg, r)
	srv := proxy.NewProxyServer(cfg, r, cm, resolver)

	return &ProxyEnv{Cfg: cfg, AppDir: appDir, certMgr: cm, resolver: resolver, Server: srv}, nil
}

// RunProxy is the entrypoint for the bare `snirect` command. It bootstraps
// the proxy and blocks until a signal or proxy failure.
func RunProxy(cmd *cobra.Command, opts RunOptions) error {
	env, err := BootstrapProxy(cmd)
	if err != nil {
		return err
	}
	defer env.Cleanup()

	// Bind the listener synchronously: the real port is known here (no race even
	// when Config.Server.Port == 0), so PAC/banner can use it immediately.
	if err := env.Server.Listen(); err != nil {
		return err
	}
	port := env.Cfg.Server.Port

	if shouldSetProxy(cmd, env.Cfg) {
		sysproxy.SetPAC(pacURL("127.0.0.1", port))
		env.pacSet = true
	}
	if opts.OnReady != nil {
		opts.OnReady(port)
	}

	// Serve in the background; report its outcome on proxyErr.
	proxyErr := make(chan error, 1)
	go func() { proxyErr <- env.Server.Serve() }()

	// Block until a signal or the proxy fails.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	var runErr error
	select {
	case err := <-proxyErr:
		runErr = fmt.Errorf("proxy failed: %w", err)
	case <-sig:
		logger.Control().Info("shutting down")
	}

	// Centralized teardown. Shutdown is a safe no-op on an already-stopped server.
	sctx, scancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer scancel()
	_ = env.Server.Shutdown(sctx)
	return runErr
}

func setupLogging(cfg *config.Config, cmd *cobra.Command) {
	level := cfg.Log.Level
	if logLevel != "" {
		level = logLevel
	}
	logger.SetLevel(level)

	if cfg.Log.File != "" {
		logPath := cfg.Log.File
		if !filepath.IsAbs(logPath) {
			appDir, _ := config.GetAppDataDir()
			logPath = filepath.Join(appDir, logPath)
		}
		if err := logger.SetOutput(logPath); err != nil {
			fmt.Fprintf(os.Stderr, "failed to set log file: %v\n", err)
		} else {
			logger.Control().Info("log file configured", "path", logPath)
		}
	}
}

func startPprof() {
	if !pprof {
		return
	}
	go func() {
		logger.Control().Info("pprof listening", "addr", pprofAddr)
		if err := http.ListenAndServe(pprofAddr, nil); err != nil {
			logger.Control().Error("pprof failed", "error", err)
		}
	}()
}

func installCA(cfg *config.Config, appDir string) {
	p := certPath(appDir)
	switch cfg.CAInstall {
	case "never":
		logger.Control().Info("CA auto-install disabled")
		return
	case "always":
		installed, err := sysproxy.ForceInstallCert(p)
		if err != nil {
			logger.Control().Warn("CA install failed", "error", err)
		} else if installed {
			logger.Control().Info("CA reinstalled")
		}
	default: // "auto" or ""
		installed, err := sysproxy.InstallCert(p)
		if err != nil {
			logger.Control().Warn("CA install failed", "error", err)
		} else if installed {
			logger.Control().Info("CA installed")
		}
	}
}

func shouldSetProxy(cmd *cobra.Command, cfg *config.Config) bool {
	if cmd.Flags().Changed("set-proxy") {
		return setProxy
	}
	return cfg.SetProxy
}
