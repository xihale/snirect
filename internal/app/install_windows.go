//go:build windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"snirect/internal/logger"
)

func getBinPath() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		homeDir, _ := os.UserHomeDir()
		localAppData = filepath.Join(homeDir, "AppData", "Local")
	}
	return filepath.Join(localAppData, "Programs", "snirect", "snirect.exe")
}

func installServicePlatform(binPath string) {
	taskName := "Snirect"

	cmd := exec.Command("schtasks", "/Create", "/TN", taskName, "/TR", fmt.Sprintf(`"%s" --set-proxy`, binPath),
		"/SC", "ONLOGON", "/RL", "HIGHEST", "/F")

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warn("创建计划任务失败: %v, 输出: %s", err, string(output))
		logger.Info("您可以手动创建启动任务或在登录时运行 snirect。")
		return
	}

	logger.Info("已创建计划任务: %s", taskName)
	logger.Info("Snirect 将在登录时自动启动。")
}
