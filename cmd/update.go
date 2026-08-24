package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/xihale/snirect/service"
	"github.com/xihale/snirect/update"
)

var updateCheckOnly bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for and install updates",
	Long: `Check GitHub Releases for a newer snirect.

  snirect update --check   report only; do not download
  snirect update           download, verify SHA-256, replace the installed binary

The check uses the same Hosts-IP + empty-SNI path as the proxy, so it
works even when the proxy itself is not running. The binary is installed
to the same location as 'snirect install' and the OS service is restarted
if it is already registered.`,
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().BoolVar(&updateCheckOnly, "check", false, "Only check; do not download or install")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client := update.New()
	fmt.Println("checking GitHub Releases...")
	info, err := client.Check(ctx, Version, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return fmt.Errorf("%w\n  fallback: https://github.com/xihale/snirect/releases", err)
	}

	if !info.Newer {
		fmt.Printf("snirect %s is up to date (latest %s)\n", Version, info.Latest)
		return nil
	}

	fmt.Printf("current: %s\n", Version)
	fmt.Printf("latest:  %s\n", info.Latest)
	if info.URL != "" {
		fmt.Printf("release: %s\n", info.URL)
	}

	if updateCheckOnly {
		fmt.Println("update available (rerun without --check to install)")
		return nil
	}

	if info.AssetURL == "" {
		return fmt.Errorf("no release asset for %s/%s\n  download from %s", runtime.GOOS, runtime.GOARCH, info.URL)
	}

	tmpDir, err := os.MkdirTemp("", "snirect-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	tmp := filepath.Join(tmpDir, info.AssetName)

	fmt.Printf("downloading %s...\n", info.AssetName)
	if err := client.Download(ctx, info, tmp); err != nil {
		return err
	}
	fmt.Println("sha256 ok")

	installed, running, _ := service.ServiceStatus()
	if running {
		fmt.Println("stopping service...")
		if err := service.StopService(); err != nil {
			return err
		}
	}

	fmt.Printf("installing to %s...\n", service.BinPath())
	if err := service.ReplaceBinary(tmp); err != nil {
		if running {
			_ = service.StartService()
		}
		return err
	}

	if installed {
		fmt.Println("starting service...")
		if err := service.StartService(); err != nil {
			return fmt.Errorf("installed %s but failed to start service: %w", info.Latest, err)
		}
	}

	fmt.Printf("updated to %s\n", info.Latest)
	if sameExecutable(service.BinPath()) {
		fmt.Println("this process is still the old binary; restart it")
	} else if !installed {
		fmt.Println("service not registered; run 'snirect install' to add it")
	}
	return nil
}

func sameExecutable(dst string) bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	a, err := os.Stat(exe)
	if err != nil {
		return false
	}
	b, err := os.Stat(dst)
	if err != nil {
		return false
	}
	return os.SameFile(a, b)
}
