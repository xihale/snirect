//go:build darwin

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"snirect/internal/logger"
)

func getBinPath() string {
	return "/usr/local/bin/snirect"
}

func installServicePlatform(binPath string) {
	homeDir, _ := os.UserHomeDir()
	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.snirect.proxy</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>--set-proxy</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>%s/Library/Logs/snirect.log</string>
	<key>StandardErrorPath</key>
	<string>%s/Library/Logs/snirect.error.log</string>
</dict>
</plist>
`, binPath, homeDir, homeDir)

	launchDir := filepath.Join(homeDir, "Library", "LaunchAgents")
	plistPath := filepath.Join(launchDir, "com.snirect.proxy.plist")

	if err := os.MkdirAll(launchDir, 0755); err != nil {
		logger.Fatal("Failed to create LaunchAgents directory: %v", err)
	}

	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		logger.Fatal("Failed to write plist file: %v", err)
	}
	logger.Info("Created launchd plist file at: %s", plistPath)

	cmd := exec.Command("launchctl", "load", plistPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.Debug("launchctl load failed: %v, output: %s", err, string(output))
	}

	cmd = exec.Command("launchctl", "enable", fmt.Sprintf("gui/%d/com.snirect.proxy", os.Getuid()))
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.Debug("launchctl enable failed: %v, output: %s", err, string(output))
	}

	logger.Info("Snirect installed and registered (auto-start enabled).")
	fmt.Printf("\nSuccess! Binary installed at %s\n", binPath)
	fmt.Println("To start the service now, run: launchctl start com.snirect.proxy")
}
