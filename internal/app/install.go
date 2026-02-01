package app

import (
	"io"
	"os"
	"path/filepath"
	"snirect/internal/logger"
)

func Install() {
	binPath := getBinPath()

	logger.Info("Installing binary to %s...", binPath)
	if err := os.MkdirAll(filepath.Dir(binPath), 0755); err != nil {
		logger.Fatal("Failed to create bin dir: %v", err)
	}

	srcPath, err := os.Executable()
	if err != nil {
		logger.Fatal("Failed to get executable path: %v", err)
	}

	if err := copyFile(srcPath, binPath); err != nil {
		logger.Fatal("Failed to copy binary: %v", err)
	}
	if err := os.Chmod(binPath, 0755); err != nil {
		logger.Fatal("Failed to set binary permissions: %v", err)
	}

	if err := SetupCA(true); err != nil {
		logger.Warn("Certificate setup warning: %v. You may need to run 'snirect install-cert' manually.", err)
	}

	installServicePlatform(binPath)
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	return err
}
