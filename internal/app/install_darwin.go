//go:build darwin

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"snirect/internal/logger"
)

func getBinPath() string {
	return "/usr/local/bin/snirect"
}

func installServicePlatform(binPath string) error {
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
	</array>
	<key>RunAtLoad</key>
	<false/>
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
		return fmt.Errorf("创建 LaunchAgents 目录失败: %w", err)
	}

	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("写入 plist 文件失败: %w", err)
	}
	logger.Info("已创建 launchd plist 文件: %s", plistPath)

	logger.Info("Snirect 安装成功。")
	return nil
}
