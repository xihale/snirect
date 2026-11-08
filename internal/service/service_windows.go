//go:build windows

package service

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName = "Snirect"
)

func installPlatform(exe string, args []string) error {
	cmdLine := fmt.Sprintf(`"%s"`, exe)
	if len(args) > 0 {
		cmdLine = fmt.Sprintf(`"%s" %s`, exe, strings.Join(args, " "))
	}

	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("opening registry key: %w", err)
	}
	defer k.Close()

	if err := k.SetStringValue(runValueName, cmdLine); err != nil {
		return fmt.Errorf("setting registry autostart value: %w", err)
	}

	return restartPlatform()
}

func uninstallPlatform() error {
	_ = stopPlatform()

	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err == nil {
		defer k.Close()
		if err := k.DeleteValue(runValueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return fmt.Errorf("deleting registry value: %w", err)
		}
	}
	return nil
}

func startPlatform() error {
	if isRunningWindows() {
		return nil
	}

	var cmdStr string
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err == nil {
		cmdStr, _, _ = k.GetStringValue(runValueName)
		k.Close()
	}

	var exePath string
	var exeArgs []string
	if cmdStr != "" {
		exePath, exeArgs = splitCommandLine(cmdStr)
	}
	if exePath == "" {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		exePath = exe
		exeArgs = []string{"run"}
	}

	cmd := exec.Command(exePath, exeArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS | 0x08000000, // CREATE_NO_WINDOW = 0x08000000
		HideWindow:    true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawning background process: %w", err)
	}
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
	return nil
}

func stopPlatform() error {
	_ = exec.Command("taskkill", "/F", "/IM", "snirect.exe", "/FI", fmt.Sprintf("PID ne %d", os.Getpid())).Run()
	return nil
}

func restartPlatform() error {
	_ = stopPlatform()
	return startPlatform()
}

func statusPlatform() (installed, running bool, detail string) {
	var installedState bool
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err == nil {
		val, _, valErr := k.GetStringValue(runValueName)
		k.Close()
		if valErr == nil && val != "" {
			installedState = true
		}
	}

	runningState := isRunningWindows()
	if runningState {
		return installedState, true, "running"
	}
	if installedState {
		return true, false, "stopped"
	}
	return false, false, ""
}

func isRunningWindows() bool {
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq snirect.exe", "/FI", fmt.Sprintf("PID ne %d", os.Getpid()), "/FO", "CSV", "/NH").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), "snirect.exe")
}

func splitCommandLine(cmdLine string) (string, []string) {
	cmdLine = strings.TrimSpace(cmdLine)
	if strings.HasPrefix(cmdLine, "\"") {
		idx := strings.Index(cmdLine[1:], "\"")
		if idx >= 0 {
			exe := cmdLine[1 : 1+idx]
			rest := strings.TrimSpace(cmdLine[1+idx+1:])
			return exe, strings.Fields(rest)
		}
	}
	parts := strings.Fields(cmdLine)
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}
