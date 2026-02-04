package sysproxy

import (
	"runtime"
)

// CheckEnv returns a map of detected environment details.
// Platform-specific implementations in sysproxy_*.go files.
func CheckEnv() map[string]string {
	env := make(map[string]string)
	env["OS"] = runtime.GOOS
	checkEnvPlatform(env)
	return env
}

// InstallCert attempts to install the CA certificate to the system trust store.
// Returns (true, nil) if certificate was newly installed.
// Returns (false, nil) if certificate was already present.
// Platform-specific implementations in sysproxy_*.go files.
func InstallCert(certPath string) (bool, error) {
	return installCertPlatform(certPath)
}

// ForceInstallCert forces installation of the CA certificate even if already present.
// Returns (true, nil) if successful.
// Platform-specific implementations in sysproxy_*.go files.
func ForceInstallCert(certPath string) (bool, error) {
	return forceInstallCertPlatform(certPath)
}

// UninstallCert removes the CA certificate from the system trust store.
// Platform-specific implementations in sysproxy_*.go files.
func UninstallCert(certPath string) error {
	return uninstallCertPlatform(certPath)
}

// CheckCertStatus checks if the CA certificate is installed in the system trust store.
// Returns true if installed, false otherwise. Platform-specific implementations.
func CheckCertStatus(certPath string) (bool, error) {
	return checkCertStatusPlatform(certPath)
}

// SetPAC sets the system proxy auto-config URL.
// Platform-specific implementations in sysproxy_*.go files.
func SetPAC(pacURL string) {
	setPACPlatform(pacURL)
}

// ClearPAC removes the system proxy auto-config URL.
// Platform-specific implementations in sysproxy_*.go files.
func ClearPAC() {
	clearPACPlatform()
}
