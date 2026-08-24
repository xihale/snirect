//go:build windows

package service

import (
	"os"
	"path/filepath"
)

func getBinPath() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		homeDir, _ := os.UserHomeDir()
		localAppData = filepath.Join(homeDir, "AppData", "Local")
	}
	return filepath.Join(localAppData, "Programs", "snirect", "snirect.exe")
}
