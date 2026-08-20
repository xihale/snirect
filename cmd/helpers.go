package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/xihale/snirect/config"
	"github.com/xihale/snirect/logger"
)

// loadAppConfig ensures the app dir exists and loads config.toml.
//
// config.LoadConfig is intentionally lenient: on a parse error it returns a
// non-nil config (the defaults overlaid with whatever decoded before the
// failure) together with the error. We must NOT silently swallow that error —
// operating on a half-parsed config is how subtle misbehavior sneaks in. So we
// fall back to the (possibly partial) config but log a clear warning first.
func loadAppConfig() (*config.Config, string, error) {
	appDir, err := config.EnsureConfig(false)
	if err != nil {
		return nil, "", fmt.Errorf("failed to initialize configuration: %w", err)
	}
	cfg, err := config.LoadConfig(filepath.Join(appDir, "config.toml"))
	if err != nil {
		// cfg may still be non-nil (defaults + partial parse); surface the error
		// loudly and proceed with the degraded config rather than crashing.
		fmt.Fprintf(os.Stderr, "WARNING: config parse error in %s, using defaults: %v\n",
			filepath.Join(appDir, "config.toml"), err)
		logger.Control().Error("config parse error, falling back to defaults", "error", err)
		if cfg == nil {
			return nil, appDir, fmt.Errorf("config is nil: %w", err)
		}
	}
	return cfg, appDir, nil
}

func certPath(appDir string) string {
	return filepath.Join(appDir, "certs", "root.crt")
}

func certDir(appDir string) string {
	return filepath.Join(appDir, "certs")
}

// pacURL builds the PAC auto-config URL the proxy serves. It is shared by the
// `run`/`serve` lifecycle, the `proxy set` subcommand, and any other caller so
// the URL shape lives in exactly one place.
func pacURL(host string, port int) string {
	return fmt.Sprintf("http://%s:%d/pac/?t=%d", host, port, time.Now().Unix())
}
