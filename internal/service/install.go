package service

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/xihale/snirect/internal/logger"
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
// When the running binary is owned by a system package (AUR/pacman), the
// copy is skipped and the packaged systemd user unit is enabled instead, so
// ~/.local/bin never shadows package upgrades with a stale binary.
func Install() error {
	if exe, pkg, ok := OwningPackage(); ok {
		return installPackaged(exe, pkg)
	}

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

// packagedUnitPath is where distro packages (AUR) ship the systemd user unit.
// Only referenced on the OwningPackage() path, which is Linux-only at runtime.
const packagedUnitPath = "/usr/lib/systemd/user/snirect.service"

// installPackaged wires up the service for a pacman-owned binary without
// copying it anywhere. Unreachable off Linux (OwningPackage gates the call).
func installPackaged(exePath, pkg string) error {
	homeDir, _ := os.UserHomeDir()

	// A stale ~/.local/bin copy from the GitHub-binary flow would shadow the
	// packaged binary on PATH; drop it along with its unit so pacman stays in
	// charge of what runs.
	legacyBin := getBinPath()
	if _, err := os.Stat(legacyBin); err == nil {
		if err := os.Remove(legacyBin); err != nil {
			logger.System().Warn("failed to remove legacy ~/.local/bin copy; remove it manually", "error", err)
		} else {
			logger.System().Info("removed legacy binary copy", "path", legacyBin)
		}
	}

	// A user unit written by the old `snirect install` shadows the packaged
	// one; remove it so /usr/lib/systemd/user takes over.
	userUnit := filepath.Join(homeDir, ".config/systemd/user", ServiceName+".service")
	if _, err := os.Stat(userUnit); err == nil {
		_ = runSystemdUser("disable", "--now", ServiceName)
		if err := os.Remove(userUnit); err == nil {
			_ = runSystemdUser("daemon-reload")
			logger.System().Info("removed legacy user unit in favor of the packaged one", "path", userUnit)
		}
	}

	if _, err := os.Stat(packagedUnitPath); err == nil {
		if err := runSystemdUser("enable", "--now", ServiceName); err != nil {
			return fmt.Errorf("enable packaged service: %w", err)
		}
		logger.System().Info("enabled packaged user service", "package", pkg, "unit", packagedUnitPath)
		return nil
	}

	// Package predates the shipped unit (older pkgrel): register a user unit
	// pointing at the packaged binary. Still no copy, so upgrades keep working.
	logger.System().Warn("package ships no user unit; writing one for the packaged binary", "package", pkg)
	return InstallService(exePath)
}

// runSystemdUser runs systemctl in the user manager, returning its combined
// output in the error for context.
func runSystemdUser(args ...string) error {
	full := append([]string{"--user"}, args...)
	out, err := exec.Command("systemctl", full...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %s: %s", strings.Join(full, " "), strings.TrimSpace(string(out)))
	}
	return nil
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
