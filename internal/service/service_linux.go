//go:build linux

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const packagedUnitPath = "/usr/lib/systemd/user/snirect.service"

func userUnitPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user", ServiceName+".service")
}

func installPlatform(exe string, args []string) error {
	home, _ := os.UserHomeDir()
	legacyBin := filepath.Join(home, ".local", "bin", "snirect")
	if _, _, ok := OwningPackage(); ok && exe != legacyBin {
		if _, err := os.Stat(legacyBin); err == nil {
			_ = os.Remove(legacyBin)
		}
	}

	_, pkgErr := os.Stat(packagedUnitPath)
	packagedExists := pkgErr == nil

	if _, _, ok := OwningPackage(); ok && len(args) == 1 && args[0] == "run" && packagedExists {
		unit := userUnitPath()
		if _, err := os.Stat(unit); err == nil {
			_ = os.Remove(unit)
			_ = runSystemdUser("daemon-reload")
		}
		return runSystemdUser("enable", "--now", ServiceName)
	}

	unit := userUnitPath()
	if err := os.MkdirAll(filepath.Dir(unit), 0755); err != nil {
		return fmt.Errorf("creating unit dir: %w", err)
	}

	var execStart string
	if len(args) > 0 {
		execStart = fmt.Sprintf(`"%s" %s`, exe, strings.Join(args, " "))
	} else {
		execStart = fmt.Sprintf(`"%s"`, exe)
	}

	unitContent := fmt.Sprintf(`[Unit]
Description=Snirect - SNI RST bypass proxy
Documentation=https://github.com/xihale/snirect
After=network.target

[Service]
ExecStart=%s
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`, execStart)

	if err := os.WriteFile(unit, []byte(unitContent), 0644); err != nil {
		return fmt.Errorf("writing unit file: %w", err)
	}

	if err := runSystemdUser("daemon-reload"); err != nil {
		return err
	}
	return runSystemdUser("enable", "--now", ServiceName)
}

func uninstallPlatform() error {
	_ = runSystemdUser("disable", "--now", ServiceName)
	unit := userUnitPath()
	if _, err := os.Stat(unit); err == nil {
		_ = os.Remove(unit)
		_ = runSystemdUser("daemon-reload")
	}
	return nil
}

func startPlatform() error {
	return runSystemdUser("start", ServiceName)
}

func stopPlatform() error {
	return runSystemdUser("stop", ServiceName)
}

func restartPlatform() error {
	return runSystemdUser("restart", ServiceName)
}

func statusPlatform() (installed, running bool, detail string) {
	_, errUser := os.Stat(userUnitPath())
	_, errPkg := os.Stat(packagedUnitPath)
	hasUnit := errUser == nil || errPkg == nil || exec.Command("systemctl", "--user", "cat", ServiceName).Run() == nil
	if !hasUnit {
		return false, false, ""
	}

	out, err := exec.Command("systemctl", "--user", "is-active", ServiceName).Output()
	state := strings.TrimSpace(string(out))
	if err == nil && state == "active" {
		return true, true, "running"
	}
	if state == "" {
		state = "stopped"
	}
	return true, false, state
}

func runSystemdUser(args ...string) error {
	full := append([]string{"--user"}, args...)
	out, err := exec.Command("systemctl", full...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %s: %s", strings.Join(full, " "), strings.TrimSpace(string(out)))
	}
	return nil
}
