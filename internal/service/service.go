// Package service provides a unified, kardianos/service-backed service
// manager that replaces the previous per-OS hand-written installers (systemd
// unit, launchd plist, Windows scheduled task).
//
// Why kardianos/service (imported as kservice):
//   - Windows previously used a logon scheduled task launching a console exe,
//     which forced three console-hiding hacks (IsSilentLaunch, HideConsole,
//     DisableColor). A real Windows Service (SCM) makes all of those obsolete.
//   - Linux/macOS get equivalent behavior with far less hand-written code.
//
// The service runs the binary with no arguments: the bare `snirect` command
// runs the proxy in the foreground, which is exactly what a managed service
// should supervise.
package service

import (
	"fmt"

	kservice "github.com/kardianos/service"
	"github.com/xihale/snirect/internal/logger"
)

// ServiceName is the OS-level service identifier (display name varies by OS).
const ServiceName = "snirect"

// serviceConfig builds the kardianos Config for the installed binary.
func serviceConfig(binPath string) *kservice.Config {
	return &kservice.Config{
		Name:        ServiceName,
		DisplayName: "Snirect",
		Description: "Snirect - SNI RST bypass proxy",
		Executable:  binPath,
		// Let kardianos pick per-OS defaults:
		//   Linux:   systemd user unit (~/.config/systemd/user)
		//   macOS:   launchd LaunchAgent
		//   Windows: Windows Service (SCM)
		Option: kservice.KeyValue{
			// Linux: user-scoped service (matches the old ~/.config/systemd/user
			// path) so install needs no root. Windows/macOS ignore this key.
			"UserService": true,
		},
	}
}

// InstallService registers and starts the OS service for the given binary.
func InstallService(binPath string) error {
	s, err := kservice.New(serviceProgram{}, serviceConfig(binPath))
	if err != nil {
		return fmt.Errorf("build service config: %w", err)
	}

	if err := s.Install(); err != nil {
		return fmt.Errorf("install service: %w", err)
	}
	logger.System().Info("service installed", "name", ServiceName)

	// Best-effort start. Some platforms (macOS LaunchAgent) don't start on
	// demand the same way; a logged warning is acceptable.
	if err := s.Start(); err != nil {
		logger.System().Warn("service installed but not started; start it manually", "error", err)
	} else {
		logger.System().Info("service started")
	}
	return nil
}

// UninstallService stops and removes the OS kservice.
func UninstallService() error {
	s, err := kservice.New(serviceProgram{}, serviceConfig(""))
	if err != nil {
		return fmt.Errorf("build service config: %w", err)
	}

	// Best-effort stop before uninstall.
	_ = s.Stop()
	if err := s.Uninstall(); err != nil {
		return fmt.Errorf("uninstall service: %w", err)
	}
	logger.System().Info("service uninstalled", "name", ServiceName)
	return nil
}

// StopService stops the installed OS kservice. It is a no-op-shaped error if
// the service is not installed; callers should check ServiceStatus first.
func StopService() error {
	s, err := kservice.New(serviceProgram{}, serviceConfig(getBinPath()))
	if err != nil {
		return fmt.Errorf("build service config: %w", err)
	}
	if err := s.Stop(); err != nil {
		return fmt.Errorf("stop service: %w", err)
	}
	return nil
}

// StartService starts the installed OS kservice.
func StartService() error {
	s, err := kservice.New(serviceProgram{}, serviceConfig(getBinPath()))
	if err != nil {
		return fmt.Errorf("build service config: %w", err)
	}
	if err := s.Start(); err != nil {
		return fmt.Errorf("start service: %w", err)
	}
	return nil
}

// ServiceStatus reports whether the OS service is installed and running,
// reusing the existing sysproxy.ServiceState shape so status plumbing is
// unchanged.
func ServiceStatus() (installed, running bool, detail string) {
	s, err := kservice.New(serviceProgram{}, serviceConfig(""))
	if err != nil {
		return false, false, err.Error()
	}
	st, err := s.Status()
	if err != nil {
		// Status unknown usually means not installed.
		return false, false, ""
	}
	switch st {
	case kservice.StatusRunning:
		return true, true, "running"
	case kservice.StatusStopped:
		return true, false, "stopped"
	default:
		return true, false, "unknown"
	}
}

// serviceProgram implements kservice.Interface. It is a placeholder: install/
// uninstall/status only need a type that satisfies the interface; the actual
// proxy lifecycle is managed by the process the service supervisor launches
// (the bare `snirect` binary), not by Start/Stop callbacks.
type serviceProgram struct{}

func (p serviceProgram) Start(s kservice.Service) error { return nil }
func (p serviceProgram) Stop(s kservice.Service) error  { return nil }
