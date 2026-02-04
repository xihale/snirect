package sysproxy

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"os"
	"os/exec"
	"runtime"
)

// HasTool checks if a system tool is available in PATH
func HasTool(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// GetCertFingerprint returns the SHA256 fingerprint of a certificate file.
func GetCertFingerprint(certPath string) (string, error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return "", err
	}
	return GetCertFingerprintFromPEM(data)
}

// GetCertFingerprintFromPEM returns the SHA256 fingerprint of a PEM-encoded certificate.
func GetCertFingerprintFromPEM(pemData []byte) (string, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return "", errors.New("failed to parse certificate PEM")
	}
	hash := sha256.Sum256(block.Bytes)
	return hex.EncodeToString(hash[:]), nil
}

// GetCertFingerprintSHA1 returns the SHA1 fingerprint of a certificate file.
func GetCertFingerprintSHA1(certPath string) (string, error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return "", err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return "", errors.New("failed to parse certificate PEM")
	}
	hash := sha1.Sum(block.Bytes)
	return hex.EncodeToString(hash[:]), nil
}

// CheckEnv returns a map of detected system environment details.
func CheckEnv() map[string]string {
	env := make(map[string]string)
	env["OS"] = runtime.GOOS
	checkEnvPlatform(env)
	return env
}

// InstallCert installs the CA certificate to the system trust store.
// Returns true if newly installed, false if already present.
func InstallCert(certPath string) (bool, error) {
	return installCertPlatform(certPath)
}

// ForceInstallCert forces re-installation of the CA certificate.
func ForceInstallCert(certPath string) (bool, error) {
	return forceInstallCertPlatform(certPath)
}

// UninstallCert removes the CA certificate from the system trust store.
func UninstallCert(certPath string) error {
	return uninstallCertPlatform(certPath)
}

// CheckCertStatus checks if the CA certificate is installed in the system trust store.
func CheckCertStatus(certPath string) (bool, error) {
	return checkCertStatusPlatform(certPath)
}

// SetPAC sets the system proxy auto-config URL.
func SetPAC(pacURL string) {
	setPACPlatform(pacURL)
}

// ClearPAC removes the system proxy auto-config URL.
func ClearPAC() {
	clearPACPlatform()
}
