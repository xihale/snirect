//go:build windows

package sysproxy

import (
	"fmt"
	"os/exec"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"snirect/internal/logger"
)

var (
	modwininet            = windows.NewLazySystemDLL("wininet.dll")
	procInternetSetOption = modwininet.NewProc("InternetSetOptionW")

	kernel32              = windows.NewLazySystemDLL("kernel32.dll")
	user32                = windows.NewLazySystemDLL("user32.dll")
	procGetConsoleProcess = kernel32.NewProc("GetConsoleProcessList")
	procGetConsoleWindow  = kernel32.NewProc("GetConsoleWindow")
	procShowWindow        = user32.NewProc("ShowWindow")
)

const (
	INTERNET_OPTION_SETTINGS_CHANGED = 39
	INTERNET_OPTION_REFRESH          = 37
)

func checkEnvPlatform(env map[string]string) {
	tools := []string{"certutil", "reg"}
	for _, tool := range tools {
		path, err := exec.LookPath(tool)
		if err == nil {
			env["Tool_"+tool] = path
		} else {
			env["Tool_"+tool] = "not found"
		}
	}
}

func installCertPlatform(certPath string) (bool, error) {
	if isCertInstalled(certPath) {
		logger.Info("根证书已安装在系统信任库中")
		return false, nil
	}

	logger.Info("正在安装证书: %s", certPath)
	cmd := exec.Command("certutil", "-addstore", "-user", "Root", certPath)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return false, fmt.Errorf("安装证书失败: %v, 输出: %s", err, string(output))
	}

	logger.Info("证书安装成功。")
	return true, nil
}

func isCertInstalled(certPath string) bool {
	cmd := exec.Command("certutil", "-user", "-verifystore", "Root", "Snirect Root CA")
	if err := cmd.Run(); err != nil {
		return false
	}

	sha1, err := GetCertFingerprintSHA1(certPath)
	if err != nil {
		return false
	}

	cmd = exec.Command("certutil", "-user", "-verifystore", "Root", sha1)
	err = cmd.Run()
	return err == nil
}

func forceInstallCertPlatform(certPath string) (bool, error) {
	logger.Info("正在强制安装证书: %s", certPath)

	uninstallCertPlatform(certPath)
	return installCertPlatform(certPath)
}

func uninstallCertPlatform(certPath string) error {
	logger.Info("正在尝试卸载证书")

	cmd := exec.Command("certutil", "-user", "-delstore", "Root", "Snirect Root CA")
	output, err := cmd.CombinedOutput()

	if err != nil {
		outputStr := string(output)
		if strings.Contains(outputStr, "not found") || strings.Contains(outputStr, "No certificates") {
			logger.Info("未找到证书")
			return nil
		}
		return fmt.Errorf("卸载证书失败: %v, 输出: %s", err, outputStr)
	}

	logger.Info("证书卸载成功。")
	return nil
}

func checkCertStatusPlatform(certPath string) (bool, error) {
	installed := isCertInstalled(certPath)
	return installed, nil
}

func setPACPlatform(pacURL string) {
	logger.Info("正在将系统代理设置为: %s", pacURL)

	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		registry.SET_VALUE)
	if err != nil {
		logger.Warn("打开注册表失败: %v", err)
		return
	}
	defer key.Close()

	if err := key.SetStringValue("AutoConfigURL", pacURL); err != nil {
		logger.Warn("设置 AutoConfigURL 失败: %v", err)
		return
	}

	notifyProxyChange()

	logger.Info("系统代理设置成功。")
}

func clearPACPlatform() {
	logger.Info("正在清除系统代理设置...")

	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		registry.SET_VALUE)
	if err != nil {
		logger.Debug("打开注册表失败: %v", err)
		return
	}
	defer key.Close()

	if err := key.DeleteValue("AutoConfigURL"); err != nil {
		logger.Debug("删除 AutoConfigURL 失败: %v", err)
	}

	notifyProxyChange()

	logger.Info("系统代理已清除。")
}

func notifyProxyChange() {
	for i := 0; i < 3; i++ {
		procInternetSetOption.Call(0, uintptr(INTERNET_OPTION_SETTINGS_CHANGED), 0, 0)
		procInternetSetOption.Call(0, uintptr(INTERNET_OPTION_REFRESH), 0, 0)
		if i < 2 {
			windows.SleepEx(100, false)
		}
	}
}

func isLaunchedBySystemOrGUIPlatform() bool {
	var ids [2]uint32
	ret, _, _ := procGetConsoleProcess.Call(
		uintptr(unsafe.Pointer(&ids[0])),
		uintptr(2),
	)

	return ret <= 1
}

func hideConsolePlatform() {
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd == 0 {
		return
	}

	procShowWindow.Call(hwnd, windows.SW_HIDE)
}

func isSilentLaunchPlatform() bool {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	getTickCount64 := kernel32.NewProc("GetTickCount64")
	uptime, _, _ := getTickCount64.Call()

	if uptime < 180000 {
		logger.Debug("Detected silent launch: System uptime is less than 3 minutes (%d ms)", uptime)
		return true
	}

	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snapshot)

	var procEntry windows.ProcessEntry32
	procEntry.Size = uint32(unsafe.Sizeof(procEntry))

	myPID := uint32(windows.GetCurrentProcessId())
	var parentPID uint32
	found := false

	err = windows.Process32First(snapshot, &procEntry)
	for err == nil {
		if procEntry.ProcessID == myPID {
			parentPID = procEntry.ParentProcessID
			found = true
			break
		}
		err = windows.Process32Next(snapshot, &procEntry)
	}

	if !found {
		return false
	}

	err = windows.Process32First(snapshot, &procEntry)
	for err == nil {
		if procEntry.ProcessID == parentPID {
			parentName := windows.UTF16ToString(procEntry.ExeFile[:])
			parentName = strings.ToLower(parentName)
			if strings.Contains(parentName, "svchost.exe") ||
				strings.Contains(parentName, "taskhostw.exe") ||
				strings.Contains(parentName, "services.exe") {
				logger.Debug("Detected silent launch: Parent process is %s", parentName)
				return true
			}
			logger.Debug("Not a silent launch: Parent process is %s, uptime is %d ms", parentName, uptime)
			break
		}
		err = windows.Process32Next(snapshot, &procEntry)
	}

	return false
}

func disableColorPlatform() {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	setConsoleMode := kernel32.NewProc("SetConsoleMode")
	getStdHandle := kernel32.NewProc("GetStdHandle")

	const (
		STD_OUTPUT_HANDLE = uint32(0xfffffff5)
		STD_ERROR_HANDLE  = uint32(0xfffffff4)
	)

	handleOut, _, _ := getStdHandle.Call(uintptr(STD_OUTPUT_HANDLE))
	handleErr, _, _ := getStdHandle.Call(uintptr(STD_ERROR_HANDLE))

	if handleOut != 0 && handleOut != uintptr(windows.InvalidHandle) {
		setConsoleMode.Call(handleOut, 0)
	}
	if handleErr != 0 && handleErr != uintptr(windows.InvalidHandle) {
		setConsoleMode.Call(handleErr, 0)
	}
}
