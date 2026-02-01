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
// Platform-specific implementations in sysproxy_*.go files.
func InstallCert(certPath string) error {
	return installCertPlatform(certPath)
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
