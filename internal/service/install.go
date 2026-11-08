package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// ReplaceExecutable replaces the current running binary with src atomically.
// On Windows, the running binary is renamed aside first.
// It returns the resolved destination path.
func ReplaceExecutable(src string) (string, error) {
	dst, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolving executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(dst); err == nil {
		dst = resolved
	}

	if err := replaceFile(src, dst); err != nil {
		return "", fmt.Errorf("replacing executable: %w", err)
	}
	if err := os.Chmod(dst, 0755); err != nil {
		return "", fmt.Errorf("setting executable permissions: %w", err)
	}
	return dst, nil
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
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, 0755); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
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
