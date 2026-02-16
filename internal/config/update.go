package config

import (
	"os"
	"path/filepath"
	"time"
)

// ShouldCheckRules determines if rules check is needed based on last check time.
func ShouldCheckRules(appDir string, checkInterval time.Duration) (bool, error) {
	checkFile := filepath.Join(appDir, ".rules_check")

	info, err := os.Stat(checkFile)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}

	lastCheck := info.ModTime()
	return time.Since(lastCheck) > checkInterval, nil
}

// ShouldCheckUpdate determines if program update check is needed based on last check time.
func ShouldCheckUpdate(appDir string, checkInterval time.Duration) (bool, error) {
	checkFile := filepath.Join(appDir, ".update_check")

	info, err := os.Stat(checkFile)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}

	lastCheck := info.ModTime()
	return time.Since(lastCheck) > checkInterval, nil
}
