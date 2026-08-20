//go:build linux

package app

import (
	"os"
	"path/filepath"
)

func getBinPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".local", "bin", "snirect")
}
