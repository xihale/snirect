package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/xihale/snirect/logger"
)

// BinPath is the install location of the snirect binary on this OS.
func BinPath() string { return getBinPath() }

// ReplaceBinary copies src onto BinPath, creating parent directories.
// On Windows the existing file is renamed aside first so a running
// process (including this one) does not lock the destination.
func ReplaceBinary(src string) error {
	dst := getBinPath()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	if err := replaceFile(src, dst); err != nil {
		return fmt.Errorf("安装二进制失败: %w", err)
	}
	if err := os.Chmod(dst, 0755); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}
	return nil
}

func replaceFile(src, dst string) error {
	if runtime.GOOS == "windows" {
		return replaceFileWindows(src, dst)
	}
	return replaceFileUnix(src, dst)
}

func replaceFileUnix(src, dst string) error {
	tmp := dst + ".new"
	if err := copyFile(src, tmp); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, 0755); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func replaceFileWindows(src, dst string) error {
	old := dst + ".old"
	if _, err := os.Stat(dst); err == nil {
		_ = os.Remove(old)
		if err := os.Rename(dst, old); err != nil {
			return fmt.Errorf("could not move existing binary aside (is it running?): %w", err)
		}
	}
	if err := copyFile(src, dst); err != nil {
		if _, statErr := os.Stat(old); statErr == nil {
			_ = os.Rename(old, dst)
		}
		return err
	}
	_ = os.Remove(old)
	return nil
}

// Install copies the binary to the system PATH and sets up the service.
func Install() error {
	binPath := getBinPath()

	logger.System().Info("installing binary", "path", binPath)
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
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return InstallService(binPath)
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
