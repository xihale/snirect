//go:build windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"snirect/internal/config"
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

func installServicePlatform(binPath string) error {
	taskName := "Snirect"
	logPath := config.GetDefaultLogPath()

	// Ensure log directory exists
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		logger.Warn("Failed to create log directory: %v", err)
	}

	cmd := exec.Command("schtasks", "/Create", "/TN", taskName, "/TR", fmt.Sprintf(`"cmd /c \"%s\" >> \"%s\" 2>&1"`, binPath, logPath),
		"/SC", "ONLOGON", "/RL", "HIGHEST", "/F")

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warn("创建计划任务失败: %v, 输出: %s", err, string(output))
		logger.Info("您可以手动创建启动任务或在登录时运行 snirect。")
		return fmt.Errorf("failed to create scheduled task: %w", err)
	}

	logger.Info("已创建计划任务: %s", taskName)
	logger.Info("Snirect 安装成功。")
	return nil
}
