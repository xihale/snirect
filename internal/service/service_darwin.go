//go:build darwin

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const plistLabel = "com.xihale.snirect"

func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", plistLabel+".plist")
}

func guiDomain() string {
	return fmt.Sprintf("gui/%d", os.Getuid())
}

func isInstalled() bool {
	_, err := os.Stat(plistPath())
	return err == nil
}

func installPlatform(exe string, args []string) error {
	p := plistPath()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return fmt.Errorf("creating LaunchAgents dir: %w", err)
	}

	var argItems strings.Builder
	fmt.Fprintf(&argItems, "\t\t<string>%s</string>\n", exe)
	for _, arg := range args {
		fmt.Fprintf(&argItems, "\t\t<string>%s</string>\n", arg)
	}

	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
%s	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
</dict>
</plist>
`, plistLabel, argItems.String())

	if err := os.WriteFile(p, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("writing plist file: %w", err)
	}

	_ = exec.Command("launchctl", "bootout", guiDomain(), p).Run()
	if _, err := exec.Command("launchctl", "bootstrap", guiDomain(), p).CombinedOutput(); err != nil {
		if out2, err2 := exec.Command("launchctl", "load", "-w", p).CombinedOutput(); err2 != nil {
			return fmt.Errorf("launchctl load: %s (%w)", strings.TrimSpace(string(out2)), err2)
		}
	}
	return nil
}

func uninstallPlatform() error {
	p := plistPath()
	_ = exec.Command("launchctl", "bootout", guiDomain(), p).Run()
	_ = exec.Command("launchctl", "unload", p).Run()
	_ = os.Remove(p)
	return nil
}

func startPlatform() error {
	if !isInstalled() {
		return fmt.Errorf("service not installed; run: snirect service install")
	}
	target := guiDomain() + "/" + plistLabel
	if err := exec.Command("launchctl", "kickstart", "-k", target).Run(); err != nil {
		if err2 := exec.Command("launchctl", "start", plistLabel).Run(); err2 != nil {
			return fmt.Errorf("launchctl start: %w", err2)
		}
	}
	return nil
}

func stopPlatform() error {
	target := guiDomain() + "/" + plistLabel
	if err := exec.Command("launchctl", "kill", "SIGTERM", target).Run(); err != nil {
		if err2 := exec.Command("launchctl", "stop", plistLabel).Run(); err2 != nil {
			return fmt.Errorf("launchctl stop: %w", err2)
		}
	}
	return nil
}

func restartPlatform() error {
	target := guiDomain() + "/" + plistLabel
	if err := exec.Command("launchctl", "kickstart", "-k", target).Run(); err != nil {
		_ = stopPlatform()
		return startPlatform()
	}
	return nil
}

func statusPlatform() (installed, running bool, detail string) {
	if !isInstalled() {
		return false, false, ""
	}
	out, err := exec.Command("launchctl", "print", guiDomain()+"/"+plistLabel).CombinedOutput()
	if err == nil && strings.Contains(string(out), "state = running") {
		return true, true, "running"
	}
	return true, false, "stopped"
}
