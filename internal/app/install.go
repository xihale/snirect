package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"snirect/internal/logger"
)

// Install copies the binary to the system PATH and sets up the service.
func Install() error {
	binPath := getBinPath()

	logger.Info("Installing binary to %s...", binPath)
	if err := os.MkdirAll(filepath.Dir(binPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	srcPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取执行文件路径失败: %w", err)
	}

	if err := copyFile(srcPath, binPath); err != nil {
		return fmt.Errorf("复制文件失败: %w", err)
	}
	if err := os.Chmod(binPath, 0755); err != nil {
		return fmt.Errorf("设置文件权限失败: %w", err)
	}

	return installServicePlatform(binPath)
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
