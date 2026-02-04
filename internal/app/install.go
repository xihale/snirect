package app

import (
	"io"
	"os"
	"path/filepath"
	"snirect/internal/logger"
)

func Install() {
	binPath := getBinPath()

	logger.Info("正在安装二进制文件到 %s...", binPath)
	if err := os.MkdirAll(filepath.Dir(binPath), 0755); err != nil {
		logger.Fatal("创建目录失败: %v", err)
	}

	srcPath, err := os.Executable()
	if err != nil {
		logger.Fatal("获取执行文件路径失败: %v", err)
	}

	if err := copyFile(srcPath, binPath); err != nil {
		logger.Fatal("复制文件失败: %v", err)
	}
	if err := os.Chmod(binPath, 0755); err != nil {
		logger.Fatal("设置文件权限失败: %v", err)
	}

	// CA certificate will be auto-generated on first run if needed
	// (via importca = "auto" in config.toml)
	// To manually install: snirect install-cert

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
