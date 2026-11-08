package service

import (
	"fmt"
	"os"
)

// ServiceName is the OS-level service identifier.
const ServiceName = "snirect"

// Options configures service installation.
type Options struct {
	SetProxy bool
}

// Install registers and starts the user background service.
func Install(opts Options) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable: %w", err)
	}
	args := []string{"run"}
	if opts.SetProxy {
		args = append(args, "-s")
	}
	return installPlatform(exe, args)
}

// Uninstall stops and removes the user background service.
func Uninstall() error {
	return uninstallPlatform()
}

// Start starts the user background service.
func Start() error {
	return startPlatform()
}

// Stop stops the user background service.
func Stop() error {
	return stopPlatform()
}

// Restart restarts the user background service.
func Restart() error {
	return restartPlatform()
}

// Status reports service installation and execution status.
func Status() (installed, running bool, detail string) {
	return statusPlatform()
}
